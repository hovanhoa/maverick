package testhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
)

// RequestBuilder is a convenience wrapper around http.Request that lets
// the user incrementally build test requests in a way suitable for writing
// unit and integration tests at scale.
type RequestBuilder struct {
	method  string
	path    string
	body    io.Reader
	header  http.Header
	cookies []*http.Cookie
}

// NewRequestBuilder create a new RequestBuilder with the required fields,
// a request method and relative path.
func NewRequestBuilder(method string, path string) RequestBuilder {
	return RequestBuilder{method, path, nil, make(http.Header), nil}
}

// WithHeader returns a RequestBuilder setting the specified key-value pair
// as a header, replacing any previously-set value for the same key.
func (r RequestBuilder) WithHeader(key, value string) RequestBuilder {
	r.header.Set(key, value)
	return r
}

// WithBody returns a new RequestBuilder replacing the body of the request
// with the given body.
func (r RequestBuilder) WithBody(body io.Reader) RequestBuilder {
	r.body = body
	return r
}

func (r RequestBuilder) WithBodyString(body string) RequestBuilder {
	return r.WithBodyBytes([]byte(body))
}

// WithBodyBytes returns a new RequestBuilder replacing the body of the request
// with the given body bytes.
func (r RequestBuilder) WithBodyBytes(body []byte) RequestBuilder {
	return r.WithBody(bytes.NewReader(body))
}

// WithBodyBytes returns a new RequestBuilder replacing the body of the request
// with the given object marshalled into JSON format, and adds a header of
// Content-type: application/json
func (r RequestBuilder) WithBodyJSON(body interface{}) RequestBuilder {
	b, err := json.Marshal(body)
	// This should only be used in tests, so it's okay to panic here
	if err != nil {
		panic(err)
	}

	return r.
		WithBodyBytes(b).
		WithHeader("Content-Type", "application/json; charset=utf-8")
}

// WithCookie returns a new RequestBuilder adding the specified cookie to the
// request.
func (r RequestBuilder) WithCookie(c http.Cookie) RequestBuilder {
	r.cookies = append(r.cookies, &c)
	return r
}

func (r RequestBuilder) WithAuthorization(token auth.JWT) RequestBuilder {
	r.header.Set("Authorization", "Bearer "+string(token.Token))
	return r
}

// Build an http.Request object to be sent to the HTTP tester.
func (r RequestBuilder) Build() *http.Request {
	tr := httptest.NewRequest(r.method, r.path, r.body)
	tr.Header = r.header

	for _, cookie := range r.cookies {
		tr.AddCookie(cookie)
	}

	return tr
}

// RequestAsserter wraps a request sent to an HTTP service under test
// and provides some methods to perform common assertions against it.
type RequestAsserter struct {
	t   *testing.T
	req *http.Request
}

// AssertMethod asserts that the request method equals the given method.
func (r RequestAsserter) AssertMethod(method string) RequestAsserter {
	assert.Equal(r.t, method, r.req.Method)
	return r
}

// AssertPathEquals asserts that the request path equals the given path.
func (r RequestAsserter) AssertPathEquals(path string) RequestAsserter {
	assert.Equal(r.t, path, r.req.URL.Path)
	return r
}
