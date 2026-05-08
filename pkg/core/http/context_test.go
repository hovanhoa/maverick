package http_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	llmhttp "github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
)

func TestContextWithRequestHeaders(t *testing.T) {
	t.Run("with_authorization", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer token123")

		ctx := llmhttp.ContextWithRequestHeaders(context.Background(), req)
		assert.Equal(t, "Bearer token123", ctx.Value(llmhttp.AuthorizationContextKey))
	})

	t.Run("with_session_cookie", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set(llmhttp.GetSessionCookieID(), "session-abc")

		ctx := llmhttp.ContextWithRequestHeaders(context.Background(), req)
		assert.Equal(t, "session-abc", ctx.Value(llmhttp.SessionCookieContextKey))
	})

	t.Run("empty_headers", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)

		ctx := llmhttp.ContextWithRequestHeaders(context.Background(), req)
		assert.Nil(t, ctx.Value(llmhttp.AuthorizationContextKey))
		assert.Nil(t, ctx.Value(llmhttp.SessionCookieContextKey))
	})
}

func TestSetRequestLogError(t *testing.T) {
	t.Run("with_setter", func(t *testing.T) {
		var captured error
		setter := llmhttp.RequestErrorSetter(func(err error) {
			captured = err
		})
		ctx := context.WithValue(context.Background(), llmhttp.RequestErrorSetterContextKey, setter)

		testErr := errors.New("test error")
		llmhttp.SetRequestLogError(ctx, testErr)
		assert.Equal(t, testErr, captured)
	})

	t.Run("no_setter", func(t *testing.T) {
		// Should not panic
		llmhttp.SetRequestLogError(context.Background(), errors.New("test"))
	})
}

func TestSetRequestLogExtraFields(t *testing.T) {
	t.Run("with_setter", func(t *testing.T) {
		var captured map[string]interface{}
		setter := llmhttp.RequestExtraFieldsSetter(func(fields map[string]interface{}) {
			captured = fields
		})
		ctx := context.WithValue(context.Background(), llmhttp.RequestExtraFieldsSetterContextKey, setter)

		fields := map[string]interface{}{"key": "value"}
		llmhttp.SetRequestLogExtraFields(ctx, fields)
		assert.Equal(t, fields, captured)
	})

	t.Run("no_setter", func(t *testing.T) {
		// Should not panic
		llmhttp.SetRequestLogExtraFields(context.Background(), nil)
	})
}

func TestCloneRequest(t *testing.T) {
	t.Run("clones_headers", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "https://example.com/path", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Custom", "value")

		cloned, err := llmhttp.CloneRequest(req)
		assert.NoError(t, err)
		assert.Equal(t, "POST", cloned.Method)
		assert.Equal(t, "https://example.com/path", cloned.URL.String())
		assert.Equal(t, "application/json", cloned.Header.Get("Content-Type"))
		assert.Equal(t, "value", cloned.Header.Get("X-Custom"))
	})

	t.Run("independent_headers", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://example.com", nil)
		req.Header.Set("X-Test", "original")

		cloned, err := llmhttp.CloneRequest(req)
		assert.NoError(t, err)

		cloned.Header.Set("X-Test", "modified")
		assert.Equal(t, "original", req.Header.Get("X-Test"))
		assert.Equal(t, "modified", cloned.Header.Get("X-Test"))
	})
}
