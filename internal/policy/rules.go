package policy

import (
	"encoding/base64"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

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
// it might be a secret: gateway/OpenAI-shaped API keys. Credit-card-looking
// numbers are handled separately below via creditCardPattern plus a Luhn
// checksum (RE2, Go's regexp engine, has no lookahead/backreferences, so
// that validation has to happen in Go code rather than in the pattern
// itself) - a bare "\d{16}"-shaped regex on its own flags far too many
// random 16-digit numbers that aren't actually card numbers.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bllmgw_[A-Za-z0-9]{10,}\b`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),
}

// creditCardPattern finds candidate 16-digit, optionally grouped, number
// sequences; luhnValid then filters out the ones that don't check out as
// real card numbers.
var creditCardPattern = regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`)

// luhnValid reports whether digits (spaces/dashes already stripped) passes
// the Luhn checksum that real card numbers satisfy.
func luhnValid(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func isLuhnValidCandidate(candidate string) bool {
	digits := strings.NewReplacer("-", "", " ", "").Replace(candidate)
	return luhnValid(digits)
}

// SensitiveDataRedaction replaces text that looks like a secret or PII
// with "[REDACTED]" rather than denying the request outright, unless Deny
// is set.
type SensitiveDataRedaction struct {
	// Deny, if true, denies the request outright on a match instead of
	// redacting and continuing. Off by default - the platform baseline
	// (DefaultChain) always redacts; a team opts into the stricter
	// behavior via its policy overrides (see TeamChain in default.go).
	Deny bool
}

func (r SensitiveDataRedaction) Name() string { return "sensitive_data_redaction" }

func (r SensitiveDataRedaction) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	matched := false
	for _, m := range req.Messages {
		for _, pattern := range sensitivePatterns {
			if pattern.MatchString(m.Content) {
				matched = true
				break
			}
		}
		if !matched {
			for _, candidate := range creditCardPattern.FindAllString(m.Content, -1) {
				if isLuhnValidCandidate(candidate) {
					matched = true
					break
				}
			}
		}
		if matched {
			break
		}
	}
	if !matched {
		return req, allow()
	}

	if r.Deny {
		return nil, Decision{
			Action:     ActionDeny,
			ReasonCode: "sensitive_data_denied",
			Message:    "request content appears to contain a secret or sensitive data",
		}
	}

	messages := make([]openai.Message, len(req.Messages))
	for i, m := range req.Messages {
		content := m.Content
		for _, pattern := range sensitivePatterns {
			content = pattern.ReplaceAllString(content, "[REDACTED]")
		}
		content = creditCardPattern.ReplaceAllStringFunc(content, func(candidate string) string {
			if isLuhnValidCandidate(candidate) {
				return "[REDACTED]"
			}
			return candidate
		})
		messages[i] = openai.Message{Role: m.Role, Content: content}
	}

	out := *req
	out.Messages = messages
	return &out, Decision{
		Action:     ActionRedact,
		ReasonCode: "sensitive_data_redacted",
		Message:    "request content was redacted before being sent upstream",
	}
}

// promptInjectionPatterns are illustrative regexes for language commonly
// used to try to override a model's system/developer instructions (a
// "jailbreak" attempt) - e.g. "ignore previous instructions" or "reveal
// your system prompt". Matched case-insensitively. Like the other rules in
// this file, this is a static, local pattern check: it evaluates request
// content in this process only and never forwards it to an LLM for
// classification, so it runs safely even on content that also contains
// credentials or other sensitive data (SensitiveDataRedaction handles that
// separately, in the same in-process manner).
var promptInjectionPatterns = []*regexp.Regexp{
	// "ignore/disregard/forget ... instructions/prompt/rules/settings/policy" -
	// the (above|previous|prior) qualifier is optional so "forget all
	// settings" matches just as well as "forget the previous instructions".
	regexp.MustCompile(`(?i)(ignore|disregard|forget) (all |any )?(the )?((above|previous|prior) )?(instructions|prompt|rules|settings|guidelines|policy|policies|configuration)`),
	regexp.MustCompile(`(?i)(reveal|show|print|output) (your|the) (system|hidden|original) prompt`),
	regexp.MustCompile(`(?i)you are now (in )?(developer|dan|jailbreak) mode`),
	regexp.MustCompile(`(?i)\bdan mode\b`),
	regexp.MustCompile(`(?i)act as (if )?(you have no|there are no) (restrictions|rules|filters)`),
	regexp.MustCompile(`(?i)pretend (you are|to be) .*(no restrictions|unfiltered|jailbroken)`),
	regexp.MustCompile(`(?i)override (your|the) (previous |system )?(instructions|rules|settings|policy)`),
	regexp.MustCompile(`(?i)(force|forced|make) (you |it )?to (process|comply|answer|respond|execute|run)`),
	regexp.MustCompile(`(?i)(regardless|no matter what) (of )?(any|your|the) (rules|restrictions|instructions|policy|policies|guidelines|settings)`),
}

// PromptInjection denies requests whose content matches a known
// prompt-injection / jailbreak pattern, such as an attempt to make the
// downstream model ignore its system instructions or disclose its system
// prompt. This is a simple illustration of the hook, matching the same
// pattern-based approach as BlockedPatterns - not a production-grade
// semantic detector.
type PromptInjection struct{}

func (r PromptInjection) Name() string { return "prompt_injection" }

func (r PromptInjection) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	for _, m := range req.Messages {
		for _, pattern := range promptInjectionPatterns {
			if pattern.MatchString(m.Content) {
				return nil, Decision{
					Action:     ActionDeny,
					ReasonCode: "prompt_injection",
					Message:    "request content matches a known prompt-injection pattern",
				}
			}
		}
	}

	return req, allow()
}

// credentialPatterns are precise formats for API keys, tokens, and private
// keys from this gateway and common third-party providers, plus a couple
// of generic shapes (a connection string with an embedded password, a
// "<secret-looking name> = <long random value>" assignment) for
// credentials that don't match a known provider format. A match here is
// treated as high-confidence credential exposure - see CredentialLeak,
// which denies outright rather than redacting. Like sensitivePatterns
// below, this is a static, local regex match: it never sends request
// content to an LLM (here or elsewhere) to classify it.
var credentialPatterns = []*regexp.Regexp{
	// This gateway's own API keys.
	regexp.MustCompile(`\bllmgw_[A-Za-z0-9]{10,}\b`),
	// OpenAI / Anthropic-style secret keys.
	regexp.MustCompile(`\bsk-(ant-|proj-)?[A-Za-z0-9_-]{20,}\b`),
	// AWS access key IDs and secret-key assignments.
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`(?i)aws_secret_access_key\s*[:=]\s*['"]?[A-Za-z0-9/+=]{40}['"]?`),
	// GitHub / GitLab personal access tokens.
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`),
	// Slack tokens and incoming webhooks.
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`),
	regexp.MustCompile(`https://hooks\.slack\.com/services/[A-Za-z0-9/]{20,}`),
	// Google API keys.
	regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`),
	// Stripe live secret/restricted keys.
	regexp.MustCompile(`\b(sk|rk)_live_[A-Za-z0-9]{20,}\b`),
	// SendGrid API keys.
	regexp.MustCompile(`\bSG\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\b`),
	// npm automation tokens.
	regexp.MustCompile(`\bnpm_[A-Za-z0-9]{30,}\b`),
	// PEM-encoded private key blocks.
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	// Twilio account SID / auth token.
	regexp.MustCompile(`\bAC[a-f0-9]{32}\b`),
	regexp.MustCompile(`\bSK[a-f0-9]{32}\b`),
	// Azure storage connection string.
	regexp.MustCompile(`(?i)AccountKey=[A-Za-z0-9+/]{40,}={0,2}`),
	// Firebase / Google Cloud server key.
	regexp.MustCompile(`\bAAAA[A-Za-z0-9_-]{7}:[A-Za-z0-9_-]{100,}\b`),
	// A JSON Web Token (three dot-separated base64url segments).
	regexp.MustCompile(`\bey[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
	// A bare "Authorization: Bearer <token>" header.
	regexp.MustCompile(`(?i)authorization:\s*bearer\s+[A-Za-z0-9._-]{16,}`),
	// DigitalOcean personal access / OAuth / refresh tokens.
	regexp.MustCompile(`\bdo[a-z]_v1_[a-f0-9]{64}\b`),
	// Mailgun API keys.
	regexp.MustCompile(`\bkey-[0-9a-f]{32}\b`),
	// New Relic user API keys.
	regexp.MustCompile(`\bNRAK-[A-Z0-9]{27}\b`),
	// PyPI upload tokens.
	regexp.MustCompile(`\bpypi-AgEIcHlwaS5vcmc[A-Za-z0-9_-]{50,}\b`),
	// Docker Hub personal access tokens.
	regexp.MustCompile(`\bdckr_pat_[A-Za-z0-9_-]{20,}\b`),
	// Postman API keys.
	regexp.MustCompile(`\bPMAK-[a-f0-9]{24}-[a-f0-9]{25,}\b`),
	// Supabase personal access tokens.
	regexp.MustCompile(`\bsbp_[a-f0-9]{40}\b`),
	// A Cloudflare/Datadog/Heroku-style credential named right before its
	// value, e.g. "cloudflare_api_token: <40-char token>" or "DD_API_KEY=<hex>".
	regexp.MustCompile(`(?i)(cloudflare|datadog|dd_api_key|heroku)[a-z_-]*\s*[:=]\s*['"]?[A-Za-z0-9-]{32,40}\b`),
	// A URL with a credential embedded before the host, e.g.
	// postgres://user:pass@host:5432/db.
	regexp.MustCompile(`(?i)\b\w+://[^\s:/@]+:[^\s:/@]+@[^\s/]+`),
}

// genericSecretAssignmentPattern matches a "<secret-looking name> = <value>"
// assignment not covered by one of the specific provider formats above,
// e.g. "api_key: <random-looking string>". The captured value is checked
// against isLikelyRealSecret before being treated as a match - RE2 has no
// lookahead, so filtering out placeholders like "changeme" or "xxxxxxxx"
// has to happen in Go code, not in the pattern.
var genericSecretAssignmentPattern = regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|access[_-]?token|auth[_-]?token|client[_-]?secret|password|passwd)\b\s*[:=]\s*['"]?([A-Za-z0-9_\-/+]{16,})['"]?`)

// knownSecretPlaceholders are example/placeholder values that commonly
// appear in documentation, sample .env files, and tutorials - not real
// credentials - so a match against one of these (case-insensitively) is
// ignored rather than blocked.
var knownSecretPlaceholders = map[string]bool{
	"changeme": true, "change_me": true, "changethis": true,
	"yourpasswordhere": true, "your_password_here": true,
	"yourapikeyhere": true, "your_api_key_here": true,
	"password123": true, "example": true, "placeholder": true,
	"insertyourkeyhere": true, "replaceme": true,
}

// genericSecretMinEntropy is deliberately lower than entropyThreshold
// (used by HighEntropySecret below): the "api_key ="-style keyword already
// gives strong context, so a lower confidence bar on the value itself is
// appropriate.
const genericSecretMinEntropy = 3.0

// isLikelyRealSecret filters genericSecretAssignmentPattern matches down to
// ones that plausibly are real credentials, rejecting known placeholder
// values and low-entropy runs (e.g. "aaaaaaaaaaaaaaaa" or "1234567890123456").
func isLikelyRealSecret(value string) bool {
	if knownSecretPlaceholders[strings.ToLower(value)] {
		return false
	}
	allSame := true
	for i := 1; i < len(value); i++ {
		if value[i] != value[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	return shannonEntropy(value) >= genericSecretMinEntropy
}

// shannonEntropy returns the Shannon entropy of s, in bits per character -
// a measure of how "random-looking" a string is, used by
// isLikelyRealSecret and HighEntropySecret to separate credentials from
// ordinary text.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	var counts [256]int
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	entropy := 0.0
	n := float64(len(s))
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// base64CandidatePattern finds base64-looking substrings worth decoding
// and re-checking against credentialPatterns - a cheap way to catch a
// credential that was base64-encoded (accidentally, e.g. copied out of a
// config file, or deliberately to dodge a plain-text filter) before it
// reaches an LLM provider.
var base64CandidatePattern = regexp.MustCompile(`\b[A-Za-z0-9+/]{20,}={0,2}\b`)

// decodedCredentialMatch reports whether content contains a base64-encoded
// substring that decodes to something matching a known credential format.
func decodedCredentialMatch(content string) bool {
	for _, candidate := range base64CandidatePattern.FindAllString(content, -1) {
		decoded, err := base64.StdEncoding.DecodeString(candidate)
		if err != nil || !utf8.Valid(decoded) {
			continue
		}
		text := string(decoded)
		for _, pattern := range credentialPatterns {
			if pattern.MatchString(text) {
				return true
			}
		}
	}
	return false
}

// CredentialLeak denies requests whose content matches a known credential
// format - a cloud/provider API key or token, a PEM private key block, a
// connection string with an embedded password, or a generic secret-looking
// assignment. Unlike SensitiveDataRedaction, it never redacts and forwards
// a scrubbed copy: a match denies the request outright, so the credential
// is never part of what (if anything) gets sent upstream to the provider.
type CredentialLeak struct{}

func (r CredentialLeak) Name() string { return "credential_leak" }

func (r CredentialLeak) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	for _, m := range req.Messages {
		for _, pattern := range credentialPatterns {
			if pattern.MatchString(m.Content) {
				return nil, Decision{
					Action:     ActionDeny,
					ReasonCode: "credential_leak",
					Message:    "request content appears to contain a credential and was blocked before being sent upstream",
				}
			}
		}
		if match := genericSecretAssignmentPattern.FindStringSubmatch(m.Content); match != nil && isLikelyRealSecret(match[1]) {
			return nil, Decision{
				Action:     ActionDeny,
				ReasonCode: "credential_leak",
				Message:    "request content appears to contain a credential and was blocked before being sent upstream",
			}
		}
		if decodedCredentialMatch(m.Content) {
			return nil, Decision{
				Action:     ActionDeny,
				ReasonCode: "credential_leak_encoded",
				Message:    "request content contains a base64-encoded credential and was blocked before being sent upstream",
			}
		}
	}

	return req, allow()
}

// entropyCandidatePattern extracts contiguous "token-like" runs (letters,
// digits, and the punctuation commonly found inside API keys/tokens) of at
// least entropyMinLength characters, as candidates for HighEntropySecret.
var entropyCandidatePattern = regexp.MustCompile(`[A-Za-z0-9+/_=.-]{20,}`)

// entropyThreshold is the Shannon entropy (bits per character) above which
// a token-like string is treated as a likely secret. Ordinary words,
// sentences, and even most identifiers fall well under this; base64/hex
// encoded random data (the shape of most API keys and tokens) sits above
// it - though so do some innocuous things like hashes or UUIDs, which is
// why this rule redacts rather than denies.
const entropyThreshold = 4.0

// HighEntropySecret redacts contiguous strings that look "random enough"
// to be a credential, even when they don't match any known provider format
// - a backstop for internally generated or otherwise unrecognized tokens.
// Unlike CredentialLeak, a match here only redacts: high-entropy strings
// also occur naturally (hashes, UUIDs, base64-encoded blobs), so treating
// every match as a hard deny would false-positive too often on ordinary
// requests. Known credential formats are still denied outright by
// CredentialLeak; this rule only ever sees what that one didn't already
// block.
type HighEntropySecret struct{}

func (r HighEntropySecret) Name() string { return "high_entropy_secret" }

func (r HighEntropySecret) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, Decision) {
	matched := false
	messages := make([]openai.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openai.Message{
			Role: m.Role,
			Content: entropyCandidatePattern.ReplaceAllStringFunc(m.Content, func(candidate string) string {
				if shannonEntropy(candidate) < entropyThreshold {
					return candidate
				}
				matched = true
				return "[REDACTED]"
			}),
		}
	}
	if !matched {
		return req, allow()
	}

	out := *req
	out.Messages = messages
	return &out, Decision{
		Action:     ActionRedact,
		ReasonCode: "high_entropy_secret_redacted",
		Message:    "request content contained a high-entropy string that looked like a secret and was redacted before being sent upstream",
	}
}

var (
	_ Rule = MaxPromptLength{}
	_ Rule = BlockedPatterns{}
	_ Rule = SensitiveDataRedaction{}
	_ Rule = PromptInjection{}
	_ Rule = CredentialLeak{}
	_ Rule = HighEntropySecret{}
)
