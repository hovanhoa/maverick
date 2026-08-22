package http

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Service abstracts over many details of creating and running HTTP servers
// and implements best practices, graceful shutdown, and health checking.
type Service struct {
	httpServer *http.Server
	engine     *gin.Engine
	router     *Router

	metricsServer *http.Server

	// if any function in this list returns true, the log should be dropped.
	dropLogs []func(c *Context) bool
	// if any function in this list returns true, the request/response body
	// is omitted from the canonical log line, but the line itself (status,
	// latency, etc.) is still logged. Use this for routes whose bodies
	// carry sensitive content (e.g. LLM prompts/completions) that
	// shouldn't be written to logs verbatim.
	dropBodyLogs []func(c *Context) bool
	healthFn     func() error

	isStopping bool
}

// HealthFn describes the type signature of a function that performs a health
// check and returns `nil` if the service is healthy and an error if not.
type HealthFn func() error

// NewService returns a pre-configured service with best practices and sensible
// defaults. It accepts additional options to configure and override default
// behavior.
func NewService(opts ...ServiceOption) *Service {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	metricsEngine := gin.New()

	s := &Service{
		httpServer: &http.Server{
			Addr:    ":8080",
			Handler: engine,
		},
		metricsServer: &http.Server{
			Addr:    ":9090",
			Handler: metricsEngine,
		},
		engine: engine,
		router: &Router{engine},
	}

	// this should never be an error
	if err := engine.SetTrustedProxies([]string{"10.0.0.0/8", "192.168.0.0/8"}); err != nil {
		panic(err)
	}

	// Log all request and responses
	engine.Use(MetricsHandler())
	engine.Use(RequestLogger(s))
	engine.Use(PanicHandler())

	// Setup request tracing
	engine.Use(sentrygin.New(sentrygin.Options{
		Repanic:         true,
		WaitForDelivery: false,
	}))

	// Handle route not found
	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"Error": gin.H{"Message": "not found"}})
	})

	// Handle health checking
	engine.GET("/healthz", func(c *gin.Context) {
		healthErr := fmt.Errorf("server uninitialized")
		if s.healthFn != nil {
			healthErr = s.healthFn()
		}

		status := http.StatusOK
		message := "ok"
		if healthErr != nil {
			status = http.StatusServiceUnavailable
			message = healthErr.Error()
		}

		c.JSON(status, gin.H{
			"Status":  status == http.StatusOK,
			"Message": message,
		})
	})

	// Handle serving prometheus metrics
	metricsRouter := &Router{metricsEngine}
	metricsRouter.GET("/metrics", FromHTTPHandler(
		promhttp.HandlerFor(
			apm.GetRegistry(),
			promhttp.HandlerOpts{
				Registry: apm.GetRegistry(),
			},
		),
	))

	// Drop some logs by default
	s.dropLogs = append(s.dropLogs, func(c *Context) bool {
		if c.Request.Method == "OPTIONS" {
			return true
		}

		if c.Request.Method == "GET" && c.FullPath() == "/healthz" {
			return true
		}

		return false
	})

	// Override the default settings with any provided opts
	for _, applyOpt := range opts {
		applyOpt(s)
	}

	return s
}

func (s *Service) NoRoute(handlers ...HandlerFunc) {
	s.engine.NoRoute(getGinHandlers(handlers)...)
}

// Router returns the underlying Gin instance to configure route handlers
func (s Service) Router() *Router {
	return s.router
}

// Handler returns the underlying HTTP handler for the configured routes
func (s Service) Handler() http.Handler {
	return s.engine
}

// Start the service in the background, wait half a second to ensure the service
// binds on the desired port, and then set the service to return healthy (if
// no other health check if configured).
func (s *Service) Start() error {
	chMain := make(chan error, 1)
	chMetrics := make(chan error, 1)
	go func() {
		chMain <- s.httpServer.ListenAndServe()
	}()
	go func() {
		chMetrics <- s.metricsServer.ListenAndServe()
	}()

	select {
	case <-time.After(500 * time.Millisecond):
		if s.healthFn == nil {
			s.healthFn = func() error { return nil }
		}
	case err := <-chMain:
		return err
	case err := <-chMetrics:
		return err
	}

	return nil
}

// GracefulStop performs a graceful shutdown. It first sets the service to unhealthy
// to indicate it shouldn't receive any more traffic. Then it calls the http server's
// Shutdown function which waits until all requests have drained before returning.
func (s *Service) GracefulStop() error {
	s.isStopping = true
	s.healthFn = func() error {
		return fmt.Errorf("server is shutting down")
	}

	// TODO: this currently immediately refuses new connections, which means
	// our service mesh may not have enough time to react to the instance
	// becoming unavailable and may still send requests (which will fail).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mainServerErr := s.httpServer.Shutdown(ctx)
	metricsServerErr := s.metricsServer.Shutdown(ctx)

	if mainServerErr != nil {
		return mainServerErr
	}

	if metricsServerErr != nil {
		return metricsServerErr
	}

	return nil
}

// GracefulStopOnTermination runs GracefulStop in response to a termination signal.
func (s *Service) GracefulStopOnTermination() error {
	// If SIGINT is received, we want to wait for 30 seconds to allow the service
	// to shutdown gracefully. If SIGTERM is received, we want to return immediately.
	ch := make(chan os.Signal, 3)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	sig := <-ch
	switch sig {
	case syscall.SIGINT, syscall.SIGTERM:
		return s.GracefulStop()
	case syscall.SIGKILL:
		return errors.New("sigkill")
	default:
		return errors.New("unhandled %s", sig)
	}
}

// GetServer returns the underlying server for testing purposes.
func (s *Service) GetServer() *http.Server {
	return s.httpServer
}

// IsStopping returns true if the service is in the process of shutting down
func (s *Service) IsStopping() bool {
	return s.isStopping
}
