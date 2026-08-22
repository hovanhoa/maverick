package policy

// defaultMaxPromptChars bounds a single request's total message content.
// ~100k characters is generously above what any of the supported models'
// context windows can use productively, so this only ever catches
// pathological payloads, not legitimate long prompts.
const defaultMaxPromptChars = 100_000

// DefaultChain returns the gateway's baseline policy chain: a prompt size
// guardrail, sensitive-data redaction, and an (empty by default) blocked
// keyword list a deployment can extend.
func DefaultChain(blockedPatterns ...string) *Chain {
	return NewChain(
		MaxPromptLength{MaxChars: defaultMaxPromptChars},
		BlockedPatterns{Patterns: blockedPatterns},
		SensitiveDataRedaction{},
	)
}
