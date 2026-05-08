// Package testhttp provides a framework for writing HTTP
// integration tests.
package testhttp

import (
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTTPTester wraps an HTTP service so that it can be tested end-to-end.
type HTTPTester struct {
	t        *testing.T
	service  *http.Service
	requests []RequestAsserter
}

// NewHTTPTester creates a new HTTPTester with the given service.
func NewHTTPTester(t *testing.T, service *http.Service) *HTTPTester {
	return &HTTPTester{t: t, service: service}
}

func (h HTTPTester) WithT(t *testing.T) *HTTPTester {
	h.t = t
	return &h
}

// Run a request through the HTTP service being tested. It records the
// response and return an object that lets the caller make assertions
// about the response.
func (h *HTTPTester) Run(r *http.Request) *ResponseAsserter {
	return &ResponseAsserter{h.run(r), h.t}
}

func (h *HTTPTester) run(r *http.Request) *httptest.ResponseRecorder {
	h.requests = append(h.requests, RequestAsserter{h.t, r})

	w := httptest.NewRecorder()
	h.service.Handler().ServeHTTP(w, r)

	return w
}

// AssertRequestsLen asserts that the tester received the given number
// of requests.
func (h *HTTPTester) AssertRequestsLen(expectedRequests int) *HTTPTester {
	assert.Len(h.t, h.requests, expectedRequests)
	return h
}

// Request returns the asserter for the i'th request sent to the tester.
func (h *HTTPTester) Request(i int) RequestAsserter {
	return h.requests[i]
}

// NewConnectedHTTPTester returns a tester for the given service and a
// client that is configured to send requests to the tester. This lets
// the caller inject the client into a service that needs this service
// as a dependency.
func NewConnectedHTTPTester(t *testing.T, service *http.Service) (*http.Client, *HTTPTester) {
	tester := NewHTTPTester(t, service)
	client := NewMockClient(func(req *http.Request) (*http.Response, error) {
		req, err := http.CloneRequest(req)
		if err != nil {
			return nil, err
		}

		return tester.Run(req).Result(), nil
	})

	return client, tester
}

type TestingT interface {
	require.TestingT
	Cleanup(func())
}

func NewTestHTTPServer(t TestingT, service *http.Service) env.Service {
	server := httptest.NewServer(service.GetServer().Handler)
	t.Cleanup(server.Close)

	url, err := url.Parse(server.URL)
	require.NoError(t, err)

	localPort, err := strconv.Atoi(url.Port())
	require.NoError(t, err)

	return env.Service{
		Name:         url.Hostname(),
		InternalOnly: true,
		LocalPort:    localPort,
		InternalPort: localPort,
	}
}

func NewTestHTTPServerFromHandler(t TestingT, handler nethttp.HandlerFunc) env.Service {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	url, err := url.Parse(server.URL)
	require.NoError(t, err)

	localPort, err := strconv.Atoi(url.Port())
	require.NoError(t, err)

	return env.Service{
		Name:         url.Hostname(),
		InternalOnly: true,
		LocalPort:    localPort,
		InternalPort: localPort,
	}
}
