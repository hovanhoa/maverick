package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hovanhoa/llmgateway/pkg/openai"
)

// MaxPromptLength denies requests whose total message content exceeds a
// configured character budget - a cheap guardrail against pathological or
// abusive payloads, applied before any provider call is made.
type MaxPromptLength struct {
	MaxChars int
}

func (r MaxPromptLength) Name() string { return "max_prompt_length" }

func (r MaxPromptLength) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	var total int
	for _, m := range req.Messages {
		total += len(m.Content)
	}

	if total > r.MaxChars {
		return nil, Decision{
			Action:     ActionDeny,
			ReasonCode: "prompt_too_long",
			Message:    fmt.Sprintf("prompt exceeds the maximum allowed length of %d characters", r.MaxChars),
		}
	}

	return req, allow()
}

// BlockedPatterns denies requests whose content matches any of a
// configured set of substrings, case-insensitively. This is a simple
// illustration of a content-policy hook, not a production moderation
// system - real deployments would likely swap this for a dedicated
// classifier.
type BlockedPatterns struct {
	Patterns []string
}

func (r BlockedPatterns) Name() string { return "blocked_patterns" }

func (r BlockedPatterns) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	for _, m := range req.Messages {
		lower := strings.ToLower(m.Content)
		for _, pattern := range r.Patterns {
			if pattern == "" {
				continue
			}
			if strings.Contains(lower, strings.ToLower(pattern)) {
				return nil, Decision{
					Action:     ActionDeny,
					ReasonCode: "blocked_pattern",
					Message:    "request content matches a blocked pattern",
				}
			}
		}
	}

	return req, allow()
}

// sensitivePatterns are illustrative regexes for content that looks like
// it might be a secret or PII: gateway/OpenAI-shaped API keys, and
// contiguous 16-digit sequences resembling a credit card number. This is
// deliberately narrow - a real deployment would want a much more
// thorough detector - but demonstrates the redact (vs. deny) action.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bllmgw_[A-Za-z0-9]{10,}\b`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
}

// SensitiveDataRedaction replaces text that looks like a secret or PII
// with "[REDACTED]" rather than denying the request outright.
type SensitiveDataRedaction struct{}

func (r SensitiveDataRedaction) Name() string { return "sensitive_data_redaction" }

func (r SensitiveDataRedaction) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	redacted := false
	messages := make([]openai.Message, len(req.Messages))

	for i, m := range req.Messages {
		content := m.Content
		for _, pattern := range sensitivePatterns {
			if pattern.MatchString(content) {
				redacted = true
				content = pattern.ReplaceAllString(content, "[REDACTED]")
			}
		}
		messages[i] = openai.Message{Role: m.Role, Content: content}
	}

	if !redacted {
		return req, allow()
	}

	out := *req
	out.Messages = messages
	return &out, Decision{
		Action:     ActionRedact,
		ReasonCode: "sensitive_data_redacted",
		Message:    "request content was redacted before being sent upstream",
	}
}

var (
	_ Rule = MaxPromptLength{}
	_ Rule = BlockedPatterns{}
	_ Rule = SensitiveDataRedaction{}
)
