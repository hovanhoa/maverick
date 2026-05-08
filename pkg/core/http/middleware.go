package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mailgun/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
)

const (
	AuthorizationHeaderKey string = "Authorization"
)

func MetricsHandler() gin.HandlerFunc {
	// Return a handler that measures the request information
	return func(c *gin.Context) {
		// Starting metrics
		start := time.Now()

		// Process the request
		c.Next()

		// Create metric labels
		labels := prometheus.Labels{
			"method": c.Request.Method,
			"path":   c.FullPath(),
			"status": strconv.Itoa(c.Writer.Status()),
		}

		// Publish metrics
		requestCount.With(labels).Inc()
		requestDuration.With(labels).Observe(float64(time.Since(start) / time.Second))
	}
}

type InterceptResponseWriter struct {
	gin.ResponseWriter
	data *bytes.Buffer
}

func (w *InterceptResponseWriter) Write(data []byte) (n int, err error) {
	_, _ = w.data.Write(data)
	return w.ResponseWriter.Write(data)
}

// RequestLogger logs a canonical line for each request.
func RequestLogger(s *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If this request should not be logged, skip this middleware
		for _, shouldDrop := range s.dropLogs {
			if shouldDrop(c) {
				return
			}
		}

		// ignore requests to the health check endpoint
		if strings.HasPrefix(c.Request.URL.Path, "/healthz") {
			return
		}

		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Inject a interceptor for the response so we can log it and analyze it
		c.Writer = &InterceptResponseWriter{c.Writer, new(bytes.Buffer)}

		// Read the request body in json format if it exists
		var requestJSON json.RawMessage
		if c.Request.Body != nil && strings.Contains(c.GetHeader("content-type"), "application/json") {
			requestJSON, _ = io.ReadAll(c.Request.Body)
			_ = c.Request.Body.Close()
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestJSON))
		}

		// Process request
		c.Next()

		if raw != "" {
			path = path + "?" + raw
		}

		if len(requestJSON) == 0 {
			requestJSON = nil
		}

		// Read the response body in json format if it exists
		var responseJSON json.RawMessage
		if strings.Contains(c.Writer.Header().Get("content-type"), "application/json") {
			responseJSON, _ = io.ReadAll(c.Writer.(*InterceptResponseWriter).data)
		}
		if len(responseJSON) == 0 {
			responseJSON = nil
		}

		// If there was an error, add it to the context
		err, _ := c.Get(string(RequestErrorContextKey))

		// Fields to be logged in the request log
		fields := []log.Field{
			log.Any("request", map[string]any{
				"clientId": c.ClientIP(),
				"method":   c.Request.Method,
				"path":     path,
				"headers":  c.Request.Header,
			}),
			log.Any("requestJson", requestJSON),
			log.Any("response", map[string]any{
				"latency":    time.Since(start),
				"statusCode": c.Writer.Status(),
				"headers":    c.Writer.Header(),
			}),
			log.Any("responseJson", responseJSON),
			log.Any("error", err),
		}

		// Extra fields may have been added to the context by other middleware
		extraFieldsValue, ok := c.Get(string(RequestExtraFieldsContextKey))
		if ok {
			extraFieldsMap := extraFieldsValue.(map[string]any)
			for k, v := range extraFieldsMap {
				fields = append(fields, log.Any(k, v))
			}
		}

		log.FromContext(c.Request.Context()).Info("CANONICAL-RESPONSE-LINE", fields...)
	}
}

// PanicHandler recovers, logs, and reports any panics that occur during the request.
// It returns a generic 500 error message to the user, but does not prevent other handlers
// from running (in particular, the requestLogger middleware will still run).
func PanicHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if v := recover(); v != nil {
				// Store the error to include in the log line
				if err, ok := v.(error); ok {
					c.Set(string(RequestErrorContextKey), errors.Stack(err))
				}

				// Return a generic server error to the user
				c.JSON(StatusInternalServerError, gin.H{"Error": NewInternalServerError()})
			}
		}()

		c.Next()
	}
}

func UnsetAuthCookie(writer http.ResponseWriter, cookieName string) {
	cookie := &http.Cookie{
		Name:    cookieName,
		Path:    "/",
		Expires: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	cookie.Domain = env.GetDomain().String()
	cookie.SameSite = http.SameSiteStrictMode
	cookie.HttpOnly = true
	cookie.Secure = true

	http.SetCookie(writer, cookie)
}

func SetAuthCookie(writer http.ResponseWriter, token auth.JWT, cookieName string) {
	cookie := &http.Cookie{
		Name:    cookieName,
		Value:   string(token.Token),
		Path:    "/",
		Expires: token.ExpiresAt,
	}

	cookie.Domain = env.GetDomain().String()
	cookie.SameSite = http.SameSiteStrictMode
	cookie.HttpOnly = true
	cookie.Secure = true

	http.SetCookie(writer, cookie)
}

func GetSessionCookieID() string {
	return "__Secure-OysterSessionID"
}

func GetSessionJWTCookieID(applicationNamespace string) string {
	// Use the environment to produce the cookie name to avoid collisions between
	// environments with subdomains (e.g. getcara.ai vs staging.getcara.ai or
	// nikhil.dev.oysterinc.net vs worktree.nikhil.dev.oysterinc.net).
	switch env.GetEnvironment() {
	case env.Staging:
		return "__Secure-Oyster-Staging-" + applicationNamespace + "-SessionToken"
	case env.Dev:
		devNamespace := getDevCookiePrefix()
		return "__Secure-Oyster-" + devNamespace + "-" + applicationNamespace + "-SessionToken"
	}

	// Production environment cookie name is unchanged.
	return "__Secure-Oyster-" + applicationNamespace + "-SessionToken"
}

func getDevCookiePrefix() string {
	// Replace all lowercase letters preceeded by a hyphen with its uppercase equivalent
	regex := regexp.MustCompile(`(?:^|-)[a-z]`)
	return regex.ReplaceAllStringFunc(env.GetDevNamespace(), func(match string) string {
		return strings.ToUpper(match)
	})
}
