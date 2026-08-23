package openai_test

import (
	"encoding/json"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	temp := 0.7
	maxTokens := 256
	req := openai.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []openai.Message{
			{Role: openai.RoleSystem, Content: "You are a helpful assistant."},
			{Role: openai.RoleUser, Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Stream:      true,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded openai.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, req, decoded)
}

func TestMessage_UnmarshalJSON_ArrayContent(t *testing.T) {
	t.Parallel()

	var m openai.Message
	err := json.Unmarshal([]byte(`{"role":"user","content":[{"type":"text","text":"Hello"},{"type":"text","text":"world"}]}`), &m)
	require.NoError(t, err)
	assert.Equal(t, openai.RoleUser, m.Role)
	assert.Equal(t, "Hello\nworld", m.Content)
}

func TestMessage_UnmarshalJSON_ArrayContent_SkipsNonTextParts(t *testing.T) {
	t.Parallel()

	var m openai.Message
	err := json.Unmarshal([]byte(`{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/x.png"}},{"type":"text","text":"describe this"}]}`), &m)
	require.NoError(t, err)
	assert.Equal(t, "describe this", m.Content)
}

func TestMessage_UnmarshalJSON_InvalidContentShape(t *testing.T) {
	t.Parallel()

	var m openai.Message
	err := json.Unmarshal([]byte(`{"role":"user","content":42}`), &m)
	assert.Error(t, err)
}

func TestChatCompletionRequest_JSONUnmarshal_ArrayContentInMessages(t *testing.T) {
	t.Parallel()

	var req openai.ChatCompletionRequest
	err := json.Unmarshal([]byte(`{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`), &req)
	require.NoError(t, err)
	require.Len(t, req.Messages, 1)
	assert.Equal(t, "hi", req.Messages[0].Content)
}

func TestChatCompletionResponse_JSONShape(t *testing.T) {
	t.Parallel()

	resp := openai.ChatCompletionResponse{
		ID:      "chatcmpl-1",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "gpt-4o",
		Choices: []openai.Choice{
			{Index: 0, Message: openai.Message{Role: openai.RoleAssistant, Content: "Hi there"}, FinishReason: openai.FinishReasonStop},
		},
		Usage: openai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, "chat.completion", raw["object"])
	assert.Contains(t, raw, "choices")
	assert.Contains(t, raw, "usage")
}

func TestChatCompletionChunk_JSONShape(t *testing.T) {
	t.Parallel()

	finish := openai.FinishReasonStop
	chunk := openai.ChatCompletionChunk{
		ID:      "chatcmpl-1",
		Object:  "chat.completion.chunk",
		Created: 1700000000,
		Model:   "gpt-4o",
		Choices: []openai.ChunkChoice{
			{Index: 0, Delta: openai.Delta{Content: "Hi"}, FinishReason: &finish},
		},
	}

	data, err := json.Marshal(chunk)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"object":"chat.completion.chunk"`)
	assert.Contains(t, string(data), `"finish_reason":"stop"`)
}
