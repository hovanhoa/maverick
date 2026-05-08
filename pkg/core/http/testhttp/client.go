package testhttp

import (
	"bytes"
	"io"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
)

// NewMockClient creates an HTTP client that performs the specified action
// to handle requests.
func NewMockClient(rt http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: http.NewRoundTripper(rt),
	}
}

// NewMockClientWithResponse is a convenience function that implements
// a RoundTripper that returns the specified response status code and body.
func NewMockClientWithResponse(statusCode int, body []byte) *http.Client {
	return NewMockClient(func(req *http.Request) (*http.Response, error) {
		var b []byte
		if body != nil {
			b, _ = io.ReadAll(req.Body)
		}

		log.
			FromContext(req.Context()).
			Debug(
				"intercepting http request",
				log.String("method", req.Method),
				log.String("url", req.URL.String()),
				log.Any("headers", req.Header),
				log.String("body", string(b)),
			)

		return &http.Response{
			StatusCode: statusCode,
			Request:    req,
			Body:       io.NopCloser(bytes.NewBuffer(body)),
		}, nil
	})
}
