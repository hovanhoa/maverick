// Package openai defines canonical, OpenAI-compatible request/response
// types for the gateway's /v1/chat/completions endpoint. Provider adapters
// (internal/provider) map to and from these types; nothing outside that
// package should ever need to speak a provider-specific wire format.
package openai

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
