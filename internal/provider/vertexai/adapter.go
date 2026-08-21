// Package vertexai is a scaffold for a Google Vertex AI adapter. Vertex
// calls require GCP OAuth2 service-account token exchange (via the GCP
// SDK and a real service account), which is a materially bigger lift than
// the API-key-based providers (Anthropic, OpenAI, Gemini) and is deferred
// to a follow-up. This adapter satisfies provider.Provider so it can be
// registered and routed to like any other provider, returning a clear
// error until the real integration lands.
package vertexai

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

// Config configures an Adapter. Empty for now; will carry GCP
// project/location/service-account configuration once the real
// integration is built.
type Config struct{}

// Adapter is a not-yet-implemented scaffold for Google Vertex AI.
type Adapter struct{}

// New returns a new Adapter.
func New(_ Config) *Adapter { return &Adapter{} }

// Name returns "vertexai".
func (a *Adapter) Name() string { return "vertexai" }

func (a *Adapter) notImplemented() error {
	return provider.NewError(a.Name(), provider.ErrorKindUnknown, "vertexai adapter is not implemented yet - requires GCP OAuth2 service-account auth", nil)
}

// ChatCompletion is not yet implemented.
func (a *Adapter) ChatCompletion(_ context.Context, _ *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	return nil, a.notImplemented()
}

// StreamChatCompletion is not yet implemented.
func (a *Adapter) StreamChatCompletion(_ context.Context, _ *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	return nil, a.notImplemented()
}

var _ provider.Provider = (*Adapter)(nil)
