// Package provider abstracts over upstream LLM providers behind the
// canonical, OpenAI-compatible request/response types in pkg/openai.
// Adapters (the anthropic, openai, gemini, bedrock, and vertexai
// sub-packages) implement Provider for one upstream API each.
package provider

import (
	"context"

	"github.com/hovanhoa/llmgateway/pkg/openai"
)

// Provider adapts a single upstream LLM provider to the canonical
// OpenAI-compatible request/response types.
type Provider interface {
	// Name returns the provider's identifier, as used in model routing and
	// in Team.ModelAllowlist entries (e.g. "anthropic", "openai", "gemini").
	Name() string

	// ChatCompletion performs a single non-streaming chat completion call.
	ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error)

	// StreamChatCompletion performs a streaming chat completion call. The
	// returned channel is closed when the stream ends, whether it finished
	// normally or failed; at most one StreamEvent ever carries a non-nil
	// Err, and it is always the last value sent before the channel closes.
	StreamChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (<-chan StreamEvent, error)
}

// StreamEvent is one item from a streaming chat completion: either a chunk
// or a terminal error.
type StreamEvent struct {
	Chunk *openai.ChatCompletionChunk
	// Usage carries final token accounting when an adapter can reliably
	// determine it from the stream (not all can - see each adapter's
	// StreamChatCompletion). When set, it always rides alongside the same
	// event as the finish-reason chunk, never a separate event, and is
	// purely an internal signal for usage/quota reconciliation - it is
	// never serialized to the client (internal/http only ever reads
	// Chunk/Err off this struct).
	Usage *openai.Usage
	Err   error
}

// Registry resolves a provider by its Name().
type Registry map[string]Provider

// Get returns the provider registered under name, if any.
func (r Registry) Get(name string) (Provider, bool) {
	p, ok := r[name]
	return p, ok
}
