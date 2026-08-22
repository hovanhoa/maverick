package http_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSessionJWTCookieID(t *testing.T) {
	t.Run("production", func(t *testing.T) {
		t.Setenv("ENVIRONMENT", "production")
		assert.Equal(t, "__Secure-Oyster-Agency-SessionToken", http.GetSessionJWTCookieID("Agency"))
	})

	t.Run("staging", func(t *testing.T) {
		t.Setenv("ENVIRONMENT", "staging")
		assert.Equal(t, "__Secure-Oyster-Staging-Agency-SessionToken", http.GetSessionJWTCookieID("Agency"))
	})

	t.Run("dev without worktree", func(t *testing.T) {
		t.Setenv("ENVIRONMENT", "dev")
		t.Setenv("OYSTER_USER", "nikhil")
		t.Setenv("OYSTER_WORKTREE", "")
		assert.Equal(t, "__Secure-Oyster-Dev-Nikhil-Agency-SessionToken", http.GetSessionJWTCookieID("Agency"))
	})

	t.Run("dev with worktree", func(t *testing.T) {
		t.Setenv("ENVIRONMENT", "dev")
		t.Setenv("OYSTER_USER", "nikhil")
		t.Setenv("OYSTER_WORKTREE", "eng-123")
		assert.Equal(t, "__Secure-Oyster-Dev-Nikhil-Eng-123-Agency-SessionToken", http.GetSessionJWTCookieID("Agency"))
	})

	t.Run("dev with multi-segment worktree", func(t *testing.T) {
		t.Setenv("ENVIRONMENT", "dev")
		t.Setenv("OYSTER_USER", "nikhil")
		t.Setenv("OYSTER_WORKTREE", "fix-auth-bug")
		assert.Equal(t, "__Secure-Oyster-Dev-Nikhil-Fix-Auth-Bug-Agency-SessionToken", http.GetSessionJWTCookieID("Agency"))
	})

	t.Run("default falls through to production format", func(t *testing.T) {
		t.Setenv("ENVIRONMENT", "")
		assert.Equal(t, "__Secure-Oyster-Agency-SessionToken", http.GetSessionJWTCookieID("Agency"))
	})
}

// TestRequestLogger_RedactsSensitiveHeaders asserts that Authorization and
// Cookie header values are never written to the canonical request log
// verbatim - only "[REDACTED]" should appear, never the raw secret.
func TestRequestLogger_RedactsSensitiveHeaders(t *testing.T) {
	originalLogger := log.New()
	t.Cleanup(func() { log.SetLogger(originalLogger) })

	logger, buf, err := log.NewCaptureLogger()
	require.NoError(t, err)
	log.SetLogger(logger)

	service := http.NewService()
	service.Router().GET("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[struct{}]) (*http.HandlerResponse, *http.Error) {
		return http.HandlerResponseJSON(map[string]string{"ok": "true"}), nil
	}))

	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/").
			WithHeader("Authorization", "Bearer super-secret-api-key").
			WithHeader("Cookie", "session=super-secret-session").
			Build(),
	).AssertStatusCode(http.StatusOK)

	require.Len(t, buf.Logs, 1)
	assert.NotContains(t, buf.Logs[0], "super-secret-api-key")
	assert.NotContains(t, buf.Logs[0], "super-secret-session")

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	request := entry["request"].(map[string]any)
	headers := request["headers"].(map[string]any)
	assert.Equal(t, []any{"[REDACTED]"}, headers["Authorization"])
	assert.Equal(t, []any{"[REDACTED]"}, headers["Cookie"])
}

// TestWithBodyLogDropper_OmitsBodyButKeepsCanonicalLine asserts that a
// route opted into WithBodyLogDropper still gets its canonical log line
// (status, latency), just without the request/response JSON body fields -
// for routes whose bodies carry sensitive content (e.g. LLM
// prompts/completions).
func TestWithBodyLogDropper_OmitsBodyButKeepsCanonicalLine(t *testing.T) {
	originalLogger := log.New()
	t.Cleanup(func() { log.SetLogger(originalLogger) })

	logger, buf, err := log.NewCaptureLogger()
	require.NoError(t, err)
	log.SetLogger(logger)

	service := http.NewService(http.WithBodyLogDropper(func(c *http.Context) bool {
		return c.FullPath() == "/sensitive"
	}))
	service.Router().POST("/sensitive", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[struct{}]) (*http.HandlerResponse, *http.Error) {
		return http.HandlerResponseJSON(map[string]string{"completion": "do-not-log-me"}), nil
	}))

	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("POST", "/sensitive").
			WithBodyJSON(map[string]string{"prompt": "do-not-log-me-either"}).
			Build(),
	).AssertStatusCode(http.StatusOK)

	require.Len(t, buf.Logs, 1)
	assert.NotContains(t, buf.Logs[0], "do-not-log-me")

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "CANONICAL-RESPONSE-LINE", entry["message"])
	assert.Nil(t, entry["requestJson"])
	assert.Nil(t, entry["responseJson"])
	response := entry["response"].(map[string]any)
	assert.EqualValues(t, http.StatusOK, response["statusCode"])
}

func TestPanicHandler(t *testing.T) {
	// Get the original logger
	originalLogger := log.New()
	t.Cleanup(func() {
		log.SetLogger(originalLogger)
	})

	// Create a capture logger to check which logs were logged
	logger, buf, err := log.NewCaptureLogger()
	require.NoError(t, err)
	log.SetLogger(logger)

	// Set up a server that will panic in the request handler
	service := http.NewService()
	service.Router().POST("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[struct{}]) (*http.HandlerResponse, *http.Error) {
		log.FromContext(ctx).Info("a log statement")
		panic(errors.New("test panic"))
	}))

	// Send the request and assert that a 500 response was returned
	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("POST", "/").
			WithBodyJSON(map[string]string{"Key": "Value"}).
			Build(),
	).
		AssertStatusCode(http.StatusInternalServerError).
		AssertError(
			http.TypeUnknown,
			http.CodeUnknown,
			"An internal server error occurred",
		)

	// Check the logs to see that each log was received
	type log struct {
		Level       string          `json:"log.level"`
		Message     string          `json:"message"`
		Error       string          `json:"Error"`
		RequestJSON json.RawMessage `json:"raw_message"`
	}

	require.Len(t, buf.Logs, 2)

	var infoLog log
	assert.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &infoLog))
	assert.Equal(t, "info", infoLog.Level)
	assert.Equal(t, "a log statement", infoLog.Message)
	assert.Empty(t, infoLog.Error)

	var responseLog log
	assert.NoError(t, json.Unmarshal([]byte(buf.Logs[1]), &responseLog))
	assert.Equal(t, "info", responseLog.Level)
	assert.Equal(t, "CANONICAL-RESPONSE-LINE", responseLog.Message)
	assert.Equal(t, "test panic", responseLog.Error)
}
