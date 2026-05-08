package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCaptureLogger(t *testing.T) *log.LogBuffer {
	t.Helper()
	logger, buf, err := log.NewCaptureLogger()
	require.NoError(t, err)
	log.SetLogger(logger)
	t.Cleanup(func() { log.SetLogger(nil) })
	return buf
}

func ctxWithLogger() context.Context {
	return log.ContextWithLogger(context.Background(), log.New())
}

func TestNewInstrumentedClient_ForwardsRequestAndResponse(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"echo":` + string(body) + `}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")

	reqBody := `{"hello":"world"}`
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodPost, server.URL+"/some/path", strings.NewReader(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"echo":{"hello":"world"}}`, string(respBody))

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "external service api call", entry["message"])
	assert.Equal(t, "test-svc", entry["externalService"])
	assert.Equal(t, "POST", entry["method"])
	assert.EqualValues(t, 200, entry["statusCode"])
}

func TestNewInstrumentedClient_RequestBodyRebuffered(t *testing.T) {
	buf := setupCaptureLogger(t)

	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")

	payload := `{"key":"value"}`
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodPost, server.URL+"/api", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.JSONEq(t, payload, string(receivedBody))

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	reqJSON, err := json.Marshal(entry["requestJson"])
	require.NoError(t, err)
	assert.JSONEq(t, payload, string(reqJSON))
}

func TestNewInstrumentedClient_ResponseBodyRebuffered(t *testing.T) {
	setupCaptureLogger(t)

	responsePayload := `{"items":[1,2,3]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responsePayload))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/data", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, responsePayload, string(body))
}

func TestNewInstrumentedClient_LogsResponseBodyAsJSON(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.NoError(t, err)

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	respJSON, err := json.Marshal(entry["responseJson"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"ok"}`, string(respJSON))
	assert.Nil(t, entry["responseBody"])
}

func TestNewInstrumentedClient_LogsNonJSONResponseAsString(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("plain text response"))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/text", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.NoError(t, err)

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "plain text response", entry["responseBody"])
	assert.Nil(t, entry["responseJson"])
}

func TestNewInstrumentedClient_LogsNonJSONRequestAsString(t *testing.T) {
	buf := setupCaptureLogger(t)

	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(receivedBody)
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodPost, server.URL+"/upload", bytes.NewBufferString("raw body content"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "raw body content", string(respBody), "server should receive the full request body after the client re-buffers it")

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "raw body content", entry["requestBody"])
	assert.Nil(t, entry["requestJson"])
}

func TestNewInstrumentedClient_LogsNonJSONRequestWithoutContentType(t *testing.T) {
	buf := setupCaptureLogger(t)

	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodPost, server.URL+"/upload", bytes.NewBufferString("no content type body"))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "no content type body", string(receivedBody))

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "no content type body", entry["requestBody"])
	assert.Nil(t, entry["requestJson"])
}

func TestNewInstrumentedClient_NonSuccessStatusCode(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"something broke"}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodPost, server.URL+"/fail", strings.NewReader(`{"action":"do_thing"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.EqualValues(t, 500, entry["statusCode"])

	respJSON, err := json.Marshal(entry["responseJson"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"error":"something broke"}`, string(respJSON))
}

func TestNewInstrumentedClient_LogsDurationMs(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.NoError(t, err)

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	durationMs, ok := entry["durationMs"].(float64)
	require.True(t, ok, "durationMs should be a number")
	assert.GreaterOrEqual(t, durationMs, float64(0))
}

func TestNewInstrumentedClient_EmptyResponseBody(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodDelete, server.URL+"/resource", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.EqualValues(t, 204, entry["statusCode"])
}

func TestNewInstrumentedClient_GETRequest_NoRequestJSON(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":["a","b"]}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/data", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Nil(t, entry["requestJson"], "GET requests with no body should not have requestJson")
	assert.NotNil(t, entry["responseJson"])
}

func TestNewInstrumentedClient_NormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		// Static paths are preserved
		{name: "static path", path: "/api/v1/contacts", want: "/api/v1/contacts"},
		{name: "root path", path: "/", want: "/"},
		{name: "empty string", path: "", want: ""},
		{name: "single segment", path: "/health", want: "/health"},

		// Numeric IDs
		{name: "numeric segment", path: "/users/12345/orders", want: "/users/{id}/orders"},
		{name: "numeric at end", path: "/policies/99", want: "/policies/{id}"},
		{name: "multiple numeric segments", path: "/agencies/42/clients/7", want: "/agencies/{id}/clients/{id}"},
		{name: "zero", path: "/items/0", want: "/items/{id}"},
		{name: "large number", path: "/records/99999999999", want: "/records/{id}"},

		// Standard UUIDs (8-4-4-4-12)
		{name: "lowercase uuid", path: "/contacts/550e8400-e29b-41d4-a716-446655440000/details", want: "/contacts/{id}/details"},
		{name: "uppercase uuid", path: "/files/550E8400-E29B-41D4-A716-446655440000", want: "/files/{id}"},
		{name: "mixed case uuid", path: "/tasks/550e8400-E29B-41d4-a716-446655440000", want: "/tasks/{id}"},

		// Hex-like IDs (HubSpot-style)
		{name: "short hex id", path: "/crm/v3/objects/abcdef12", want: "/crm/v3/objects/{id}"},
		{name: "long hex id", path: "/deals/1a2b3c4d5e6f7890", want: "/deals/{id}"},
		{name: "hex with dashes", path: "/objects/abcd-efgh-1234", want: "/objects/abcd-efgh-1234"}, // not pure hex — preserved

		// Mixed segments
		{name: "uuid and numeric", path: "/api/v2/agencies/550e8400-e29b-41d4-a716-446655440000/clients/42/policies", want: "/api/v2/agencies/{id}/clients/{id}/policies"},

		// Non-ID segments that look similar but should be preserved
		{name: "short alpha", path: "/api/v2/search", want: "/api/v2/search"},
		{name: "short hex below threshold", path: "/tags/abc123", want: "/tags/abc123"}, // 6 chars, under 8 threshold
		{name: "negative number", path: "/offset/-5", want: "/offset/{id}"},
		{name: "version prefix", path: "/api/v3/contacts", want: "/api/v3/contacts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, normalizePath(tt.path))
		})
	}
}

func TestNewInstrumentedClient_DefaultPathNormalizationInLog(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/agencies/550e8400-e29b-41d4-a716-446655440000/clients/42", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "/agencies/{id}/clients/{id}", entry["endpoint"])
}

func TestNewInstrumentedClient_WithPathNormalizer(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc", WithPathNormalizer(func(path string) string {
		return strings.ReplaceAll(path, "/crm/v3/objects/", "/crm/v3/objects/{type}/")
	}))
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/crm/v3/objects/contacts", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "/crm/v3/objects/{type}/contacts", entry["endpoint"])
}

func TestNewInstrumentedClient_WithPathNormalizerComposesWithDefault(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	var receivedPath string
	client := NewInstrumentedClient("test-svc", WithPathNormalizer(func(path string) string {
		receivedPath = path
		return path
	}))

	// Path has a UUID that the default normalizePath should replace with {id}
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/contacts/550e8400-e29b-41d4-a716-446655440000/details", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The custom normalizer should receive the already-normalized path
	assert.Equal(t, "/contacts/{id}/details", receivedPath)

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "/contacts/{id}/details", entry["endpoint"])
}

func TestNewInstrumentedClient_WithoutPathNormalizerUsesDefault(t *testing.T) {
	buf := setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodGet, server.URL+"/users/12345/orders", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Len(t, buf.Logs, 1)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.Logs[0]), &entry))
	assert.Equal(t, "/users/{id}/orders", entry["endpoint"])
}

func TestCloneRequestWithBody_ReplaysConsumedBodyViaGetBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	payload := `{"hello":"world"}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	cloned, err := CloneRequestWithBody(req)
	require.NoError(t, err)
	require.NotNil(t, cloned)

	body, err := io.ReadAll(cloned.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, string(body), "CloneRequestWithBody should recover the body via GetBody after Do consumed it")
}

func TestCloneRequestWithBody_NoBody(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/api", nil)
	require.NoError(t, err)

	cloned, err := CloneRequestWithBody(req)
	require.NoError(t, err)
	require.NotNil(t, cloned)

	body, err := io.ReadAll(cloned.Body)
	require.NoError(t, err)
	assert.Empty(t, body)
}

func TestCloneRequestWithBody_AfterInstrumentedClient(t *testing.T) {
	// Reproduces the production flow: a caller builds a request with a
	// replayable body (bytes.NewReader), sends it through NewInstrumentedClient
	// (which re-buffers req.Body into an io.NopCloser), wraps the failure in a
	// ServiceError, and later tries to recover the outbound payload from the
	// cloned request. This should work because (a) the instrumented transport
	// leaves req.GetBody intact and (b) CloneRequest preserves GetBody.
	setupCaptureLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Conflict - duplicate key"))
	}))
	defer server.Close()

	client := NewInstrumentedClient("test-svc")

	payload := `{"agency_id":"abc","client_id":"xyz"}`
	req, err := http.NewRequestWithContext(ctxWithLogger(), http.MethodPost, server.URL+"/api/log", bytes.NewReader([]byte(payload)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	require.NotNil(t, req.GetBody, "sanity: stdlib should auto-populate GetBody for bytes.NewReader")

	resp, err := client.Do(req)
	require.NoError(t, err)
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()

	svcErr := NewServiceError(req, resp.StatusCode, string(respBody))
	require.NotNil(t, svcErr.Request)
	require.NotNil(t, svcErr.Request.GetBody, "CloneRequest should preserve GetBody so the body can be replayed later")

	cloned, err := CloneRequestWithBody(svcErr.Request)
	require.NoError(t, err)

	gotBody, err := io.ReadAll(cloned.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, string(gotBody), "request body should be recoverable after going through NewInstrumentedClient + NewServiceError")
	assert.Equal(t, "Conflict - duplicate key", svcErr.Message, "sanity: response body is kept separately on the ServiceError")
}

func TestCloneRequest_PreservesGetBody(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", bytes.NewReader([]byte(`{"x":1}`)))
	require.NoError(t, err)
	require.NotNil(t, req.GetBody)

	cloned, err := CloneRequest(req)
	require.NoError(t, err)
	require.NotNil(t, cloned.GetBody, "clone should inherit GetBody from the source request")

	r, err := cloned.GetBody()
	require.NoError(t, err)
	defer r.Close()
	body, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, `{"x":1}`, string(body))
}

func TestCloneRequestWithBody_BodyWithoutGetBody(t *testing.T) {
	// Custom reader that isn't one of the types the stdlib auto-wraps with GetBody.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", io.NopCloser(strings.NewReader("streamed payload")))
	require.NoError(t, err)
	require.Nil(t, req.GetBody, "sanity: io.NopCloser shouldn't trigger GetBody autopopulation")

	cloned, err := CloneRequestWithBody(req)
	require.NoError(t, err)
	require.NotNil(t, cloned)

	body, err := io.ReadAll(cloned.Body)
	require.NoError(t, err)
	assert.Equal(t, "streamed payload", string(body), "an unconsumed body without GetBody should still be read from Body")
}

func TestServiceError_ErrorFormat(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/api/resource", nil)
	require.NoError(t, err)

	svcErr := NewServiceError(req, http.StatusUnauthorized, `{"error":"Invalid user name or password"}`)

	assert.Contains(t, svcErr.Error(), "HTTPError")
	assert.Contains(t, svcErr.Error(), "[401]")
	assert.Contains(t, svcErr.Error(), `{"error":"Invalid user name or password"}`)
	assert.Equal(t, http.StatusUnauthorized, svcErr.Status)
	assert.Equal(t, `{"error":"Invalid user name or password"}`, svcErr.Message)
}

func TestServiceError_IsRetryable(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
	require.NoError(t, err)

	tests := []struct {
		status    int
		retryable bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusRequestTimeout, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
	}

	for _, tt := range tests {
		svcErr := NewServiceError(req, tt.status, "body")
		assert.Equal(t, tt.retryable, svcErr.IsRetryable(), "status %d", tt.status)
	}
}
