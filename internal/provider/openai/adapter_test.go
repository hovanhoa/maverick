package openai_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	openaiprovider "github.com/hovanhoa/llmgateway/internal/provider/openai"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAdapter(t *testing.T, handler http.HandlerFunc) *openaiprovider.Adapter {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return openaiprovider.New(server.Client(), openaiprovider.Config{APIKey: "test-key", BaseURL: server.URL})
}

func chatRequest() *openai.ChatCompletionRequest {
	return &openai.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []openai.Message{{Role: openai.RoleUser, Content: "Hello"}},
	}
}

func TestChatCompletion_MapsRequestAndResponse(t *testing.T) {
	t.Parallel()

	var gotAuth, gotBody string
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)

		fmt.Fprint(w, `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "gpt-4o",
			"choices": [{"index":0,"message":{"role":"assistant","content":"Hi there"},"finish_reason":"stop"}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
		}`)
	})

	resp, err := adapter.ChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-key", gotAuth)
	assert.Contains(t, gotBody, `"model":"gpt-4o"`)
	assert.NotContains(t, gotBody, `"stream"`, "stream must be omitted, not sent as false, for a non-streaming call")

	assert.Equal(t, "chatcmpl-1", resp.ID)
	assert.Equal(t, "Hi there", resp.Choices[0].Message.Content)
	assert.Equal(t, 7, resp.Usage.TotalTokens)
}

func TestChatCompletion_AuthError(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key","type":"invalid_request_error"}}`)
	})

	_, err := adapter.ChatCompletion(context.Background(), chatRequest())
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, provider.ErrorKindAuth, perr.Kind)
}

func TestChatCompletion_QuotaError(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited","type":"rate_limit_error"}}`)
	})

	_, err := adapter.ChatCompletion(context.Background(), chatRequest())
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, provider.ErrorKindQuota, perr.Kind)
}

func TestChatCompletion_ServerErrorIsTransient(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":{"message":"overloaded","type":"server_error"}}`)
	})

	_, err := adapter.ChatCompletion(context.Background(), chatRequest())
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, provider.ErrorKindTransient, perr.Kind)
}

func TestChatCompletion_Timeout(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(w, `{}`)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := adapter.ChatCompletion(ctx, chatRequest())
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, provider.ErrorKindTimeout, perr.Kind)
}

func TestStreamChatCompletion_ForwardsChunksAndStopsAtDone(t *testing.T) {
	t.Parallel()

	sse := `data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}]}` + "\n\n" +
		"data: [DONE]\n\n"

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	})

	events, err := adapter.StreamChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)

	var texts []string
	var count int
	for ev := range events {
		require.NoError(t, ev.Err)
		count++
		texts = append(texts, ev.Chunk.Choices[0].Delta.Content)
	}

	assert.Equal(t, 2, count, "[DONE] must not be forwarded as a chunk")
	assert.Equal(t, []string{"Hel", "lo"}, texts)
}

func TestAdapter_Name(t *testing.T) {
	t.Parallel()
	adapter := openaiprovider.New(http.DefaultClient, openaiprovider.Config{APIKey: "x"})
	assert.Equal(t, "openai", adapter.Name())
}
