package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
)

type (
	Client         = http.Client
	Cookie         = http.Cookie
	Header         = http.Header
	Request        = http.Request
	Response       = http.Response
	ResponseWriter = http.ResponseWriter
)

var (
	DetectContentType     = http.DetectContentType
	NewRequest            = http.NewRequest
	NewRequestWithContext = http.NewRequestWithContext
)

type userIDKey string

const (
	AuthorizationContextKey userIDKey = "net.oysterinc.gocode.pkg.core.http.authorization"
	SessionCookieContextKey userIDKey = "net.oysterinc.gocode.pkg.core.http.session_cookie"
	InternalUserEmailKey    userIDKey = "net.oysterinc.gocode.pkg.core.http.internal_user_email"
)

// RoundTripper describes how to handle an HTTP request. It is the function
// equivalent of the http.RoundTripper interface.
type RoundTripper func(req *http.Request) (*http.Response, error)

// roundTripper wraps a RoundTripper into an http.RoundTripper
type roundTripper struct {
	RoundTripper
}

// NewRoundTripper wraps a RoundTripper into a struct that plainly
// implements http.RoundTripper.
func NewRoundTripper(rt RoundTripper) http.RoundTripper {
	return &roundTripper{rt}
}

// RoundTrip implements the http.RoundTripper interface and calls the
// underlying RoundTripper
func (m *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripper(req)
}

// NewClient returns a new HTTP client with reasonable defaults and
// metrics/logging support.
func NewClient() *Client {
	return &http.Client{
		Transport: NewRoundTripper(func(req *http.Request) (*http.Response, error) {
			ctx := req.Context()
			var span *apm.Span

			if hub := apm.GetHubFromContext(req.Context()).Clone(); hub != nil {
				ctx = apm.SetHubOnContext(req.Context(), hub)
				span = apm.StartSpan(ctx, "http.client", apm.WithDescription(fmt.Sprintf("%s %s", req.Method, req.URL.String())))
			}

			res, err := (&http.Client{Timeout: 30 * time.Minute}).Do(req.WithContext(ctx))

			if span != nil && res != nil {
				span.SetData("http_status", strconv.Itoa(res.StatusCode))
				span.Finish()
			}

			return res, err
		}),
	}
}

// CloneRequest returns a copy of the given HTTP request, copying the
// context, method, URL, and body to the new request. Note that the
// body is only copied by reference - meaning reading the body in
// either the original or copied request will modify the other.
// This means this method should only be used in cases where
// the body is only being read in one request.
//
// GetBody, if set on the source request, is preserved on the clone. This is
// important because http.NewRequestWithContext only auto-populates GetBody when
// the body argument is one of the stdlib's recognized replayable types
// (*bytes.Buffer, *bytes.Reader, *strings.Reader). When the body has already
// been wrapped (e.g. by an instrumenting round-tripper that re-buffers it in an
// io.NopCloser), that auto-detection would otherwise be lost on the clone.
func CloneRequest(req *http.Request) (*http.Request, error) {
	newReq, err := http.NewRequestWithContext(
		req.Context(),
		req.Method,
		req.URL.String(),
		req.Body,
	)
	if err != nil {
		return nil, err
	}

	newReq.Header = make(http.Header)
	for k := range req.Header {
		newReq.Header.Set(k, req.Header.Get(k))
	}

	if req.GetBody != nil {
		newReq.GetBody = req.GetBody
	}

	return newReq, nil
}

// CloneRequestWithBody reads the request body and returns a new request with the same context, method, URL, and body.
// If the current Body has already been consumed (e.g. the request has been sent via http.Client.Do, which always
// closes Request.Body), it falls back to GetBody. The stdlib populates GetBody automatically when the request was
// built with a *bytes.Buffer, *bytes.Reader, or *strings.Reader, which covers the common case of a JSON payload
// created via bytes.NewReader / strings.NewReader.
func CloneRequestWithBody(req *http.Request) (*http.Request, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	if len(body) == 0 && req.GetBody != nil {
		if r, err := req.GetBody(); err == nil && r != nil {
			body, _ = io.ReadAll(r)
			_ = r.Close()
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	return CloneRequest(req)
}

// NewServiceClient returns a client configured to talk to the given
// service in the current environment.
func NewServiceClient(service *env.Service) *Client {
	client := NewClient()
	return &http.Client{
		Transport: NewRoundTripper(func(req *http.Request) (*http.Response, error) {
			// replace the req.URL base path and scheme with service.InternalURL()
			serviceBaseURL, err := url.Parse(service.InternalURL())
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse service URL %q", service.InternalURL())
			}

			req.URL.Scheme = serviceBaseURL.Scheme
			req.URL.Host = serviceBaseURL.Host

			ctx := req.Context()
			if val := ctx.Value(InternalUserEmailKey); val != nil {
				if email, ok := val.(string); ok {
					req.Header.Add("X-Oyster-Retool-User", email)
					req.Header.Add("X-Oyster-Application-Namespace", "Admin")
				}
			}
			if val := ctx.Value(AuthorizationContextKey); val != nil {
				if authorization, ok := val.(string); ok {
					req.Header.Add(AuthorizationHeaderKey, authorization)
				}
			}
			if val := ctx.Value(SessionCookieContextKey); val != nil {
				if sessionCookie, ok := val.(string); ok {
					req.Header.Add(GetSessionCookieID(), sessionCookie)
				}
			}

			if transaction := apm.TransactionFromContext(req.Context()); transaction != nil {
				req.Header.Set(apm.TraceHeader, transaction.ToSentryTrace())
				req.Header.Set(apm.BaggageHeader, transaction.ToBaggage())

				if hub := apm.GetHubFromContext(req.Context()).Clone(); hub != nil {
					ctx = apm.SetHubOnContext(req.Context(), hub)
				}
			}

			return client.Do(req.WithContext(ctx))
		}),
	}
}

var idSegmentPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$|^[0-9a-fA-F-]{8,}$`,
)

// normalizePath converts a raw URL path into a templated route pattern
// suitable for use as a Prometheus metric label. Numeric and ID-like path
// segments (integers, UUIDs, hex strings) are replaced with "{id}" to keep
// cardinality bounded.
func normalizePath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if _, err := strconv.Atoi(p); err == nil {
			parts[i] = "{id}"
		} else if idSegmentPattern.MatchString(p) {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

type instrumentedClientOptions struct {
	pathNormalizer func(path string) string
}

type InstrumentedClientOption func(*instrumentedClientOptions)

func WithPathNormalizer(fn func(path string) string) InstrumentedClientOption {
	return func(o *instrumentedClientOptions) {
		o.pathNormalizer = fn
	}
}

// NewInstrumentedClient returns an HTTP client that automatically records
// Prometheus metrics for every outbound request. It composes with NewClient's
// APM span logic — each request gets both a Sentry span and Prometheus
// counters/histograms.
//
// The integration label is bound at construction time so every request through
// this client is tagged without per-call-site instrumentation.
func NewInstrumentedClient(externalService string, opts ...InstrumentedClientOption) *Client {
	inner := NewClient()

	options := &instrumentedClientOptions{
		pathNormalizer: normalizePath,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &http.Client{
		Transport: NewRoundTripper(func(req *http.Request) (*http.Response, error) {
			start := time.Now()

			var requestJSON json.RawMessage
			var requestBody []byte
			if req.Body != nil {
				if strings.Contains(req.Header.Get("content-type"), "application/json") {
					requestJSON, _ = io.ReadAll(req.Body)
					_ = req.Body.Close()
					req.Body = io.NopCloser(bytes.NewBuffer(requestJSON))
				} else {
					requestBody, _ = io.ReadAll(req.Body)
					_ = req.Body.Close()
					req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
				}
			}

			resp, err := inner.Do(req)
			duration := time.Since(start)

			statusCode := 0
			if resp != nil {
				statusCode = resp.StatusCode
			}

			path := normalizePath(req.URL.Path)
			if options.pathNormalizer != nil {
				path = options.pathNormalizer(path)
			}

			httpClientRequestTotal.WithLabelValues(externalService, req.Method, strconv.Itoa(statusCode), path).Inc()
			httpClientRequestDuration.WithLabelValues(externalService, req.Method, strconv.Itoa(statusCode), path).Observe(duration.Seconds())

			logger := log.FromContext(req.Context())
			fields := []log.Field{
				log.String("externalService", externalService),
				log.String("method", req.Method),
				log.String("endpoint", path),
				log.Int("statusCode", statusCode),
				log.Int64("durationMs", int64(duration.Milliseconds())),
			}

			if len(requestJSON) > 0 {
				fields = append(fields, log.JSON("requestJson", requestJSON))
			} else if len(requestBody) > 0 {
				fields = append(fields, log.String("requestBody", string(requestBody)))
			}

			if resp != nil && resp.Body != nil {
				// Read the response body and reset it
				responseJSON, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewBuffer(responseJSON))

				// Log the response body as JSON if the content type is application/json
				contentType := resp.Header.Get("Content-Type")
				if strings.Contains(contentType, "application/json") {
					fields = append(fields, log.JSON("responseJson", responseJSON))
				} else {
					fields = append(fields, log.String("responseBody", string(responseJSON)))
				}
			}

			logger.Info("external service api call", fields...)
			return resp, err
		}),
	}
}

// ServiceError represents an error response from a service.
type ServiceError struct {
	// Request is the HTTP request that caused the error.
	Request *Request
	// Status is the HTTP status code returned by the API.
	Status int
	// Message is response body from the API.
	Message string
}

func NewServiceError(request *Request, status int, payload string) *ServiceError {
	req, _ := CloneRequest(request)
	return &ServiceError{
		Request: req,
		Status:  status,
		Message: payload,
	}
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("HTTPError: [%d] - %s", e.Status, e.Message)
}

func (e *ServiceError) IsRetryable() bool {
	return IsRetryableStatusCode(e.Status)
}

func IsRetryableStatusCode(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusRequestTimeout ||
		statusCode >= http.StatusInternalServerError
}
