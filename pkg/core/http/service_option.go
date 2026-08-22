package http

import (
	"github.com/gin-contrib/cors"
)

// ServiceOption injects additional options into the service to customize or
// override default behavior.
type ServiceOption func(s *Service)

// WithHealthCheckFn sets the health check function that the service should
// use to determine if it should receive traffic or not.
func WithHealthCheckFn(healthFn HealthFn) ServiceOption {
	return func(s *Service) {
		s.setHealthFn(healthFn)
	}
}

type CORSOptionFunc func(cors.Config) cors.Config

// WithCORS configures the service to allow CORS
func WithCORS(fn ...CORSOptionFunc) ServiceOption {
	if len(fn) > 1 {
		panic("WithCORS only accepts a single config")
	}

	if len(fn) == 0 {
		fn = []CORSOptionFunc{func(c cors.Config) cors.Config { return c }}
	}

	return func(s *Service) {
		s.engine.Use(cors.New(fn[0](cors.Config{
			AllowOriginFunc: func(origin string) bool {
				return true
			},
			AllowCredentials: true,
			AllowMethods:     []string{"GET", "PATCH", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{
				"Authorization",
				"Content-Type",
				"Content-Encoding",
				"Referer",
				"User-Agent",
				"X-Oyster-Application-Namespace",
				"X-Merchant-API-Key",
				"X-Merchant-Integration-ID",
				"baggage",
				"sentry-trace",
				"traceparent",
				"tracestate",
			},
		})))
	}
}

// WithBindAddress overrides the service listener address to the given
// host:port combination
func WithBindAddress(addr string) ServiceOption {
	return func(s *Service) {
		s.httpServer.Addr = addr
	}
}

// WithMetricsBindAddress overrides the metrics listener address to the given
// host:port combination
func WithMetricsBindAddress(addr string) ServiceOption {
	return func(s *Service) {
		s.metricsServer.Addr = addr
	}
}

// WithLogDropper sets an option on the service that allows the user to
// configure which requests logs should be dropped for. Note, this only
// impacts request logs and not any other logs that are explicitly logged
// while handling the request. The passed function should return true for
// requests for which logs should be dropped.
func WithLogDropper(fn func(c *Context) bool) ServiceOption {
	return func(s *Service) {
		s.dropLogs = append(s.dropLogs, fn)
	}
}

// WithBodyLogDropper configures which requests should have their
// request/response body omitted from the canonical log line, while still
// logging the line itself (status, latency, headers). Use this for routes
// whose bodies carry sensitive content that shouldn't be written to logs
// verbatim (e.g. LLM prompts/completions), as opposed to WithLogDropper
// which drops the entire log line.
func WithBodyLogDropper(fn func(c *Context) bool) ServiceOption {
	return func(s *Service) {
		s.dropBodyLogs = append(s.dropBodyLogs, fn)
	}
}
