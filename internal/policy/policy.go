// Package policy implements pre-request content guardrails for the LLM
// proxy: a chain of Rules, each able to allow, redact, or deny a request
// before it ever reaches a provider.
//
// The baseline rules in rules.go are deliberately simple - pattern/length
// checks, not ML-based moderation - illustrating the hook structure the
// plan calls for rather than a production-grade safety system.
package policy

import "github.com/hovanhoa/llmgateway/pkg/openai"

// Action is what a Rule decided to do about a request.
type Action string

const (
	ActionAllow  Action = "allow"
	ActionRedact Action = "redact"
	ActionDeny   Action = "deny"
)

// Decision is the outcome of evaluating one Rule (or an entire Chain)
// against a request. Message is meant for the caller (returned in the
// proxy error response on Deny); it must never echo back raw request
// content - see ReasonCode for what gets logged.
type Decision struct {
	Action     Action
	ReasonCode string
	Message    string
}

func allow() Decision { return Decision{Action: ActionAllow} }

// Rule inspects a request and returns a (possibly modified, for redaction)
// copy of it along with a Decision. Rules must not mutate req in place -
// callers pass the working copy through a chain of rules and rely on each
// rule returning its own copy when redacting.
type Rule interface {
	Name() string
	Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision)
}

// Chain runs a fixed sequence of Rules against a request. The first Deny
// short-circuits the chain; Redact decisions accumulate (the redacted
// request produced by one rule is what subsequent rules see) rather than
// short-circuiting, since multiple independent redactions can coexist.
type Chain struct {
	rules []Rule
}

// NewChain returns a Chain that runs the given rules in order.
func NewChain(rules ...Rule) *Chain {
	return &Chain{rules: rules}
}

// Evaluate runs every rule in the chain against req. On Deny, it returns
// immediately with a nil request and that rule's Decision. Otherwise it
// returns the (possibly redacted) working request and either the last
// Redact decision seen or an Allow decision if no rule redacted anything.
func (c *Chain) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	working := req
	decision := allow()

	for _, rule := range c.rules {
		next, d := rule.Evaluate(working)
		switch d.Action {
		case ActionDeny:
			return nil, d
		case ActionRedact:
			working = next
			decision = d
		case ActionAllow:
			working = next
		}
	}

	return working, decision
}
