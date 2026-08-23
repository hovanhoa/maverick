// Package openai defines canonical, OpenAI-compatible request/response
// types for the gateway's /v1/chat/completions endpoint. Provider adapters
// (internal/provider) map to and from these types; nothing outside that
// package should ever need to speak a provider-specific wire format.
package openai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Role identifies who authored a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a chat completion request or response.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// contentPart is one element of the OpenAI "array of content parts" form of
// a message's content (e.g. `[{"type":"text","text":"..."}]`). Only the
// "text" part type is honored - image/audio parts are silently dropped,
// since this gateway does not support multimodal input; a text-only client
// still gets a sensible (if incomplete) prompt rather than a hard failure.
type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// UnmarshalJSON accepts content as either a plain string - the common case,
// and the only form this type's own MarshalJSON ever produces - or an
// OpenAI-style array of content parts. Some OpenAI-compatible clients
// (Continue, Cursor, and other agentic IDEs) send the array form even for
// pure-text messages, e.g. once a conversation has tool output or code
// context attached to a turn.
func (m *Message) UnmarshalJSON(data []byte) error {
	var raw struct {
		Role    Role            `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Role = raw.Role

	if len(raw.Content) == 0 {
		return nil
	}

	var asString string
	if err := json.Unmarshal(raw.Content, &asString); err == nil {
		m.Content = asString
		return nil
	}

	var parts []contentPart
	if err := json.Unmarshal(raw.Content, &parts); err != nil {
		return fmt.Errorf("message content must be a string or an array of content parts: %w", err)
	}

	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Type == "text" || p.Type == "" {
			texts = append(texts, p.Text)
		}
	}
	m.Content = strings.Join(texts, "\n")
	return nil
}

// ChatCompletionRequest is the canonical, OpenAI-compatible request body
// for /v1/chat/completions.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// FinishReason describes why a completion stopped generating.
type FinishReason string

const (
	FinishReasonStop   FinishReason = "stop"
	FinishReasonLength FinishReason = "length"
)

// Usage reports token accounting for a completed (non-streaming) request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Choice is a single completion candidate. The gateway only ever requests
// and returns one (n=1 is assumed throughout).
type Choice struct {
	Index        int          `json:"index"`
	Message      Message      `json:"message"`
	FinishReason FinishReason `json:"finish_reason"`
}

// ChatCompletionResponse is the canonical, OpenAI-compatible response body
// for a non-streaming /v1/chat/completions call.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Delta carries the incremental content of one streaming chunk.
type Delta struct {
	Role    Role   `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChunkChoice is a single choice within a streaming chunk.
type ChunkChoice struct {
	Index        int           `json:"index"`
	Delta        Delta         `json:"delta"`
	FinishReason *FinishReason `json:"finish_reason,omitempty"`
}

// ChatCompletionChunk is one server-sent event payload of a streaming
// /v1/chat/completions call, matching OpenAI's chat.completion.chunk shape.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}
