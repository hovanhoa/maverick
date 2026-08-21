package gemini_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/provider/gemini"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAdapter(t *testing.T, handler http.HandlerFunc) *gemini.Adapter {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return gemini.New(server.Client(), gemini.Config{APIKey: "test-key", BaseURL: server.URL})
}

func chatRequest() *openai.ChatCompletionRequest {
	return &openai.ChatCompletionRequest{
		Model: "gemini-1.5-pro",
		Messages: []openai.Message{
			{Role: openai.RoleSystem, Content: "Be terse."},
			{Role: openai.RoleUser, Content: "Hello"},
		},
	}
}

func TestChatCompletion_MapsRequestAndResponse(t *testing.T) {
	t.Parallel()

	var gotPath, gotBody string
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)

		fmt.Fprint(w, `{
			"candidates": [{"content":{"parts":[{"text":"Hi there"}],"role":"model"},"finishReason":"STOP"}],
			"usageMetadata": {"promptTokenCount": 8, "candidatesTokenCount": 2, "totalTokenCount": 10}
		}`)
	})

	resp, err := adapter.ChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)

	assert.Contains(t, gotPath, "/v1beta/models/gemini-1.5-pro:generateContent")
	assert.Contains(t, gotPath, "key=test-key")
	assert.Contains(t, gotBody, `"systemInstruction":{"parts":[{"text":"Be terse."}]}`)
	assert.Contains(t, gotBody, `"role":"user"`)
	assert.NotContains(t, gotBody, `"role":"system"`)

	assert.Equal(t, "Hi there", resp.Choices[0].Message.Content)
	assert.Equal(t, openai.FinishReasonStop, resp.Choices[0].FinishReason)
	assert.Equal(t, 10, resp.Usage.TotalTokens)
}

func TestChatCompletion_AssistantRoleMapsToModel(t *testing.T) {
	t.Parallel()

	var gotBody string
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}],"role":"model"},"finishReason":"STOP"}]}`)
	})

	req := &openai.ChatCompletionRequest{
		Model: "gemini-1.5-pro",
		Messages: []openai.Message{
			{Role: openai.RoleUser, Content: "hi"},
			{Role: openai.RoleAssistant, Content: "hello"},
		},
	}
	_, err := adapter.ChatCompletion(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, gotBody, `"role":"model"`)
}

func TestChatCompletion_MaxTokensFinishReason(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"..."}],"role":"model"},"finishReason":"MAX_TOKENS"}]}`)
	})

	resp, err := adapter.ChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)
	assert.Equal(t, openai.FinishReasonLength, resp.Choices[0].FinishReason)
}

func TestChatCompletion_AuthError(t *testing.T) {
	t.Parallel()

	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"code":401,"message":"API key not valid","status":"UNAUTHENTICATED"}}`)
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
		fmt.Fprint(w, `{"error":{"code":429,"message":"quota exceeded","status":"RESOURCE_EXHAUSTED"}}`)
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
		fmt.Fprint(w, `{"error":{"code":503,"message":"overloaded","status":"UNAVAILABLE"}}`)
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

func TestStreamChatCompletion_ForwardsIncrementalChunks(t *testing.T) {
	t.Parallel()

	sse := `data: {"candidates":[{"content":{"parts":[{"text":"Hel"}],"role":"model"}}]}` + "\n\n" +
		`data: {"candidates":[{"content":{"parts":[{"text":"lo"}],"role":"model"},"finishReason":"STOP"}]}` + "\n\n"

	var gotPath string
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	})

	events, err := adapter.StreamChatCompletion(context.Background(), chatRequest())
	require.NoError(t, err)

	var texts []string
	var sawFinish bool
	for ev := range events {
		require.NoError(t, ev.Err)
		texts = append(texts, ev.Chunk.Choices[0].Delta.Content)
		if ev.Chunk.Choices[0].FinishReason != nil {
			sawFinish = true
		}
	}

	assert.Contains(t, gotPath, "streamGenerateContent")
	assert.Contains(t, gotPath, "alt=sse")
	assert.Equal(t, []string{"Hel", "lo"}, texts)
	assert.True(t, sawFinish)
}

func TestAdapter_Name(t *testing.T) {
	t.Parallel()
	adapter := gemini.New(http.DefaultClient, gemini.Config{APIKey: "x"})
	assert.Equal(t, "gemini", adapter.Name())
}
