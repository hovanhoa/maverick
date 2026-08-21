// Package proxy implements the business logic behind the gateway's
// OpenAI-compatible LLM proxy endpoint: request validation, provider
// routing (by a "provider/model" prefix on the request's model field),
// the Phase 2 per-team model allowlist check, and dispatch to the
// resolved provider.Provider, with a retry wrapper on non-streaming calls.
//
// This package is deliberately independent of the HTTP framework -
// internal/http wires it into an actual route.
package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

// maxRetries bounds how many additional attempts a non-streaming call gets
// when the upstream provider returns a transient error.
const maxRetries = 2

// Dependencies of the Handler.
type Dependencies struct {
	Database  *db.Database
	Providers provider.Registry
}

// Handler implements the proxy's request handling, independent of any HTTP
// framework.
type Handler struct {
	deps Dependencies
}

// NewHandler returns a new Handler.
func NewHandler(deps Dependencies) *Handler {
	return &Handler{deps: deps}
}

// Principal is the concrete Principal instantiation used throughout this
// project - see internal/model/identity.go.
type Principal = auth.Principal[model.Identity, model.Role]

// resolve splits a "provider/model" request model field and looks up the
// named provider in the registry.
func (h *Handler) resolve(modelField string) (provider.Provider, string, string, error) {
	providerName, modelName, ok := strings.Cut(modelField, "/")
	if !ok || providerName == "" || modelName == "" {
		return nil, "", "", provider.NewError("", provider.ErrorKindInvalidRequest,
			`model must be formatted as "provider/model", e.g. "anthropic/claude-3-5-sonnet-20241022"`, nil)
	}

	p, ok := h.deps.Providers.Get(providerName)
	if !ok {
		return nil, "", "", provider.NewError(providerName, provider.ErrorKindInvalidRequest,
			fmt.Sprintf("unknown provider %q", providerName), nil)
	}

	return p, providerName, modelName, nil
}

// checkAllowlist enforces the Phase 2 per-team model allowlist. An account
// with no team is unrestricted, matching the Phase 2 decision that the
// allowlist is a team-level control.
func (h *Handler) checkAllowlist(ctx context.Context, principal *Principal, providerName, modelName string) error {
	if principal.OrgID == "" {
		return nil
	}

	team, err := h.deps.Database.GetTeamByID(ctx, principal.OrgID)
	if err != nil {
		return err
	}
	if team == nil {
		return nil
	}

	if !team.IsModelAllowed(providerName, modelName) {
		return provider.NewError(providerName, provider.ErrorKindPolicy,
			fmt.Sprintf("model %q is not on this team's allowlist", providerName+"/"+modelName), nil)
	}

	return nil
}

// prepare validates the request, resolves its provider/model, and checks
// the caller's team allowlist. It returns the provider to call and a copy
// of the request with Model rewritten to the bare upstream model name
// (the "provider/" prefix is a gateway routing concern, not something any
// adapter should ever see).
func (h *Handler) prepare(ctx context.Context, principal *Principal, req *openai.ChatCompletionRequest) (provider.Provider, *openai.ChatCompletionRequest, error) {
	if err := req.Validate(); err != nil {
		return nil, nil, provider.NewError("", provider.ErrorKindInvalidRequest, err.Error(), nil)
	}

	p, providerName, modelName, err := h.resolve(req.Model)
	if err != nil {
		return nil, nil, err
	}

	if err := h.checkAllowlist(ctx, principal, providerName, modelName); err != nil {
		return nil, nil, err
	}

	upstream := *req
	upstream.Model = modelName
	return p, &upstream, nil
}

// ChatCompletion handles a single non-streaming chat completion call.
func (h *Handler) ChatCompletion(ctx context.Context, principal *Principal, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	p, upstream, err := h.prepare(ctx, principal, req)
	if err != nil {
		return nil, err
	}

	return provider.WithRetry(ctx, maxRetries, func() (*openai.ChatCompletionResponse, error) {
		return p.ChatCompletion(ctx, upstream)
	})
}

// StreamChatCompletion handles a streaming chat completion call. Streaming
// calls are not retried - a client already receiving chunks can't safely
// have the request silently restarted underneath it.
func (h *Handler) StreamChatCompletion(ctx context.Context, principal *Principal, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	p, upstream, err := h.prepare(ctx, principal, req)
	if err != nil {
		return nil, err
	}

	return p.StreamChatCompletion(ctx, upstream)
}
