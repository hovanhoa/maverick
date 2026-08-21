package anthropic_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/provider/anthropic"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAdapter(t *testing.T, handler http.HandlerFunc) *anthropic.Adapter {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return anthropic.New(server.Client(), anthropic.Config{APIKey: "test-key", BaseURL: server.URL})
}

func chatRequest() *openai.ChatCompletionRequest {
	return &openai.ChatCompletionRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []openai.Message{
			{Role: openai.RoleSystem, Content: "Be terse."},
			{Role: openai.RoleUser, Content: "Hello"},
		},
	}
}

func TestChatCompletion_MapsRequestAndResponse(t *testing.T) {
	t.Parallel()

	var gotAuth, gotVersion, gotBody string
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "msg_123",
			"model": "claude-3-5-sonnet-20241022",
			"content": [{"type":"text","text":"Hi there"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 3}
		}`)
	})

	resp, err := adapter.ChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)

	assert.Equal(t, "test-key", gotAuth)
	assert.Equal(t, "2023-06-01", gotVersion)
	assert.Contains(t, gotBody, `"system":"Be terse."`)
	assert.NotContains(t, gotBody, `"role":"system"`)

	assert.Equal(t, "msg_123", resp.ID)
	assert.Equal(t, "Hi there", resp.Choices[0].Message.Content)
	assert.Equal(t, openai.RoleAssistant, resp.Choices[0].Message.Role)
	assert.Equal(t, openai.FinishReasonStop, resp.Choices[0].FinishReason)
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 3, resp.Usage.CompletionTokens)
	assert.Equal(t, 13, resp.Usage.TotalTokens)
}

func TestChatCompletion_MaxTokensFinishReason(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"msg_1","model":"claude-3","content":[{"type":"text","text":"..."}],"stop_reason":"max_tokens","usage":{"input_tokens":1,"output_tokens":1}}`)
	})

	resp, err := adapter.ChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)
	assert.Equal(t, openai.FinishReasonLength, resp.Choices[0].FinishReason)
}

func TestChatCompletion_AuthError(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	})

	_, err := adapter.ChatCompletion(context.Background(), chatRequest())
	require.Error(t, err)

	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, provider.ErrorKindAuth, perr.Kind)
	assert.Contains(t, perr.Message, "invalid x-api-key")
}

func TestChatCompletion_RateLimitError(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"slow down"}}`)
	})

	_, err := adapter.ChatCompletion(context.Background(), chatRequest())
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, provider.ErrorKindQuota, perr.Kind)
}

func TestChatCompletion_ServerErrorIsTransient(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"type":"api_error","message":"overloaded"}}`)
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

func TestStreamChatCompletion_ForwardsChunksInOrder(t *testing.T) {
	t.Parallel()

	sse := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_abc","model":"claude-3-5-sonnet-20241022"}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hel"}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"lo"}}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}` + "\n\n" +
		"event: message_stop\n" +
		`data: {"type":"message_stop"}` + "\n\n"

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sse)
	})

	events, err := adapter.StreamChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)

	var texts []string
	var sawFinish bool
	for ev := range events {
		require.NoError(t, ev.Err)
		require.NotNil(t, ev.Chunk)
		assert.Equal(t, "msg_abc", ev.Chunk.ID)
		if ev.Chunk.Choices[0].Delta.Content != "" {
			texts = append(texts, ev.Chunk.Choices[0].Delta.Content)
		}
		if ev.Chunk.Choices[0].FinishReason != nil {
			sawFinish = true
			assert.Equal(t, openai.FinishReasonStop, *ev.Chunk.Choices[0].FinishReason)
		}
	}

	assert.Equal(t, []string{"Hel", "lo"}, texts)
	assert.True(t, sawFinish)
}

func TestStreamChatCompletion_NonOKStatusReturnsError(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"type":"authentication_error","message":"bad key"}}`)
	})

	_, err := adapter.StreamChatCompletion(context.Background(), chatRequest())
	require.Error(t, err)
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, provider.ErrorKindAuth, perr.Kind)
}

func TestAdapter_Name(t *testing.T) {
	t.Parallel()
	adapter := anthropic.New(http.DefaultClient, anthropic.Config{APIKey: "x"})
	assert.Equal(t, "anthropic", adapter.Name())
}
