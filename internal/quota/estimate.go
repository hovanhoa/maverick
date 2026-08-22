package quota

import "github.com/hovanhoa/llmgateway/pkg/openai"

// defaultCompletionEstimate is used when a request doesn't specify
// max_tokens, so there's no upper bound to reserve against.
const defaultCompletionEstimate = 1000

// charsPerTokenEstimate approximates English text as ~4 characters per
// token, a common rough heuristic when no real tokenizer is available.
const charsPerTokenEstimate = 4

// EstimateTokens returns a pre-call estimate of how many tokens a request
// will consume, for sizing a quota reservation. It is deliberately rough -
// prompt tokens are approximated from character count, and completion
// tokens are capped by the request's max_tokens (or a default estimate
// when unset) since the actual output length isn't knowable in advance.
// The estimate is only ever used to reserve quota; Checker.Reconcile
// corrects it to the provider's actual reported usage after the call.
func EstimateTokens(req *openai.ChatCompletionRequest) int {
	var chars int
	for _, m := range req.Messages {
		chars += len(m.Content)
	}

	promptEstimate := chars / charsPerTokenEstimate
	if promptEstimate == 0 && chars > 0 {
		promptEstimate = 1
	}

	completionEstimate := defaultCompletionEstimate
	if req.MaxTokens != nil {
		completionEstimate = *req.MaxTokens
	}

	return promptEstimate + completionEstimate
}
