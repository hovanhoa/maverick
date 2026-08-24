package policy

// defaultMaxPromptChars bounds a single request's total message content.
// ~100k characters is generously above what any of the supported models'
// context windows can use productively, so this only ever catches
// pathological payloads, not legitimate long prompts.
const defaultMaxPromptChars = 100_000

// DefaultChain returns the gateway's baseline policy chain: a prompt size
// guardrail, an (empty by default) blocked keyword list a deployment can
// extend, a prompt-injection / jailbreak check, a hard block on known
// credential formats, redaction of softer PII (e.g. credit-card-looking
// numbers, validated via Luhn), and finally a high-entropy-string backstop
// for credentials that don't match any known format. CredentialLeak runs
// ahead of SensitiveDataRedaction so a high-confidence credential match
// denies the request outright instead of being redacted and still
// forwarded upstream; HighEntropySecret runs last, as the most speculative
// and priciest check, and only ever sees what the earlier rules didn't
// already catch.
func DefaultChain(blockedPatterns ...string) *Chain {
	return NewChain(
		MaxPromptLength{MaxChars: defaultMaxPromptChars},
		BlockedPatterns{Patterns: blockedPatterns},
		PromptInjection{},
		CredentialLeak{},
		SensitiveDataRedaction{},
		HighEntropySecret{},
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
