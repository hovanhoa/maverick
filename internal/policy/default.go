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

// TeamOverrides carries a team's policy customizations. It only adds
// deny-type checks on top of the platform baseline (DefaultChain) - see
// TeamChain and internal/proxy/proxy.go's prepare(), which runs a team's
// TeamChain ahead of the baseline chain so a stricter deny always sees
// the original, unredacted content rather than whatever the baseline
// chain already redacted.
type TeamOverrides struct {
	BlockedPatterns     []string
	DenyOnSensitiveData bool
}

// HasOverrides reports whether o customizes anything, so callers can skip
// building and running a chain for the common case of an unconfigured
// team.
func (o TeamOverrides) HasOverrides() bool {
	return len(o.BlockedPatterns) > 0 || o.DenyOnSensitiveData
}

// TeamChain returns a policy chain for a team's overrides alone. It is
// meant to run ahead of DefaultChain, not in place of it - a team can only
// make policy stricter than the platform baseline, never weaker.
func TeamChain(o TeamOverrides) *Chain {
	var rules []Rule
	if len(o.BlockedPatterns) > 0 {
		rules = append(rules, BlockedPatterns{Patterns: o.BlockedPatterns})
	}
	if o.DenyOnSensitiveData {
		rules = append(rules, SensitiveDataRedaction{Deny: true})
	}
	return NewChain(rules...)
}
