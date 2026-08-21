package openai

import "github.com/hovanhoa/llmgateway/pkg/core/errors"

// Validate checks structural and range constraints on a chat completion
// request. It does not know anything about which provider/model the
// request will be routed to - that's checked separately (model allowlist,
// provider availability).
func (r *ChatCompletionRequest) Validate() error {
	if r.Model == "" {
		return errors.New("model is required")
	}
	if len(r.Messages) == 0 {
		return errors.New("messages must not be empty")
	}
	for i, m := range r.Messages {
		switch m.Role {
		case RoleSystem, RoleUser, RoleAssistant:
		default:
			return errors.New("messages[%d].role must be one of system, user, assistant", i)
		}
		if m.Content == "" {
			return errors.New("messages[%d].content must not be empty", i)
		}
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return errors.New("temperature must be between 0 and 2")
	}
	if r.TopP != nil && (*r.TopP < 0 || *r.TopP > 1) {
		return errors.New("top_p must be between 0 and 1")
	}
	if r.MaxTokens != nil && *r.MaxTokens <= 0 {
		return errors.New("max_tokens must be positive")
	}

	return nil
}
