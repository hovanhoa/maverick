package testhttp_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/stretchr/testify/assert"
)

func TestRequestBuilder(t *testing.T) {
	req := testhttp.NewRequestBuilder("GET", "/path").Build()
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/path", req.URL.Path)
	assert.Empty(t, req.Header)

	// Test a body reader
	req = testhttp.NewRequestBuilder("POST", "/path").
		WithBody(bytes.NewBuffer([]byte(`test`))).
		Build()
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/path", req.URL.Path)
	assert.Empty(t, req.Header)

	data, err := io.ReadAll(req.Body)
	assert.NoError(t, err)
	assert.Equal(t, []byte(`test`), data)

	// Test a body string
	req = testhttp.NewRequestBuilder("POST", "/path").
		WithBodyString(`test`).
		Build()
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/path", req.URL.Path)
	assert.Empty(t, req.Header)

	data, err = io.ReadAll(req.Body)
	assert.NoError(t, err)
	assert.Equal(t, []byte(`test`), data)

	// Test a body bytes
	req = testhttp.NewRequestBuilder("POST", "/path").
		WithBodyBytes([]byte(`test`)).
		Build()
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/path", req.URL.Path)
	assert.Empty(t, req.Header)

	data, err = io.ReadAll(req.Body)
	assert.NoError(t, err)
	assert.Equal(t, []byte(`test`), data)

	// Test a JSON body (success)
	req = testhttp.NewRequestBuilder("POST", "/path").
		WithBodyJSON(map[string]interface{}{"A": "a", "B": 2}).
		Build()
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/path", req.URL.Path)
	assert.Equal(t, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, req.Header)

	data, err = io.ReadAll(req.Body)
	assert.NoError(t, err)
	assert.Equal(t, []byte(`{"A":"a","B":2}`), data)

	// Test a malformed JSON body
	assert.Panics(t, func() {
		testhttp.NewRequestBuilder("POST", "/path").
			WithBodyJSON(make(chan int)).
			Build()
	})

	// Test headers
	req = testhttp.NewRequestBuilder("POST", "/path").
		WithHeader("X-Test-1", "1").
		WithHeader("X-Test-2", "2").
		WithAuthorization(auth.JWT{ID: "test", Token: "test", ExpiresAt: time.Now().Add(1 * time.Hour)}).
		WithCookie(http.Cookie{Name: "C", Value: "3"}).
		WithCookie(http.Cookie{Name: "D", Value: "4"}).
		Build()
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/path", req.URL.Path)
	assert.Equal(t, http.Header{
		"X-Test-1":      []string{"1"},
		"X-Test-2":      []string{"2"},
		"Authorization": []string{"Bearer test"},
		"Cookie":        []string{"C=3; D=4"},
	}, req.Header)
}
