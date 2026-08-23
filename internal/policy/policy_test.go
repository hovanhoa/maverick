package policy_test

import (
	"strings"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/policy"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func req(content string) *openai.ChatCompletionRequest {
	return &openai.ChatCompletionRequest{
		Model:    "anthropic/claude-3-5-sonnet",
		Messages: []openai.Message{{Role: openai.RoleUser, Content: content}},
	}
}

func TestMaxPromptLength_Allows(t *testing.T) {
	t.Parallel()

	rule := policy.MaxPromptLength{MaxChars: 100}
	out, d := rule.Evaluate(req("short"))
	assert.Equal(t, policy.ActionAllow, d.Action)
	assert.NotNil(t, out)
}

func TestMaxPromptLength_Denies(t *testing.T) {
	t.Parallel()

	rule := policy.MaxPromptLength{MaxChars: 10}
	out, d := rule.Evaluate(req(strings.Repeat("x", 20)))
	assert.Equal(t, policy.ActionDeny, d.Action)
	assert.Equal(t, "prompt_too_long", d.ReasonCode)
	assert.Nil(t, out)
}

func TestBlockedPatterns_DeniesCaseInsensitively(t *testing.T) {
	t.Parallel()

	rule := policy.BlockedPatterns{Patterns: []string{"forbidden-word"}}
	_, d := rule.Evaluate(req("this contains FORBIDDEN-WORD in it"))
	assert.Equal(t, policy.ActionDeny, d.Action)
	assert.Equal(t, "blocked_pattern", d.ReasonCode)
}

func TestBlockedPatterns_AllowsCleanContent(t *testing.T) {
	t.Parallel()

	rule := policy.BlockedPatterns{Patterns: []string{"forbidden-word"}}
	_, d := rule.Evaluate(req("perfectly normal request"))
	assert.Equal(t, policy.ActionAllow, d.Action)
}

func TestSensitiveDataRedaction_RedactsApiKeyLookingStrings(t *testing.T) {
	t.Parallel()

	rule := policy.SensitiveDataRedaction{}
	out, d := rule.Evaluate(req("my key is llmgw_abcdefghijklmnop, please help"))
	assert.Equal(t, policy.ActionRedact, d.Action)
	assert.Equal(t, "sensitive_data_redacted", d.ReasonCode)
	assert.Contains(t, out.Messages[0].Content, "[REDACTED]")
	assert.NotContains(t, out.Messages[0].Content, "llmgw_abcdefghijklmnop")
}

func TestSensitiveDataRedaction_RedactsCreditCardLookingStrings(t *testing.T) {
	t.Parallel()

	rule := policy.SensitiveDataRedaction{}
	out, d := rule.Evaluate(req("card: 4111 1111 1111 1111"))
	assert.Equal(t, policy.ActionRedact, d.Action)
	assert.NotContains(t, out.Messages[0].Content, "4111")
}

func TestSensitiveDataRedaction_LeavesCleanContentAlone(t *testing.T) {
	t.Parallel()

	rule := policy.SensitiveDataRedaction{}
	original := req("nothing sensitive here")
	out, d := rule.Evaluate(original)
	assert.Equal(t, policy.ActionAllow, d.Action)
	assert.Same(t, original, out, "unmodified requests should be returned as-is, not copied")
}

func TestSensitiveDataRedaction_DenyMode_DeniesInsteadOfRedacting(t *testing.T) {
	t.Parallel()

	rule := policy.SensitiveDataRedaction{Deny: true}
	out, d := rule.Evaluate(req("my key is llmgw_abcdefghijklmnop, please help"))
	assert.Equal(t, policy.ActionDeny, d.Action)
	assert.Equal(t, "sensitive_data_denied", d.ReasonCode)
	assert.Nil(t, out)
}

func TestSensitiveDataRedaction_DenyMode_AllowsCleanContent(t *testing.T) {
	t.Parallel()

	rule := policy.SensitiveDataRedaction{Deny: true}
	_, d := rule.Evaluate(req("nothing sensitive here"))
	assert.Equal(t, policy.ActionAllow, d.Action)
}

func TestTeamOverrides_HasOverrides(t *testing.T) {
	t.Parallel()

	assert.False(t, policy.TeamOverrides{}.HasOverrides())
	assert.True(t, policy.TeamOverrides{BlockedPatterns: []string{"x"}}.HasOverrides())
	assert.True(t, policy.TeamOverrides{DenyOnSensitiveData: true}.HasOverrides())
}

func TestTeamChain_DeniesOnExtraBlockedPattern(t *testing.T) {
	t.Parallel()

	chain := policy.TeamChain(policy.TeamOverrides{BlockedPatterns: []string{"company-secret-project"}})
	_, d := chain.Evaluate(req("tell me about company-secret-project"))
	assert.Equal(t, policy.ActionDeny, d.Action)
}

func TestTeamChain_DeniesOnSensitiveDataWhenOptedIn(t *testing.T) {
	t.Parallel()

	chain := policy.TeamChain(policy.TeamOverrides{DenyOnSensitiveData: true})
	_, d := chain.Evaluate(req("my key is llmgw_abcdefghijklmnop"))
	assert.Equal(t, policy.ActionDeny, d.Action)
}

func TestTeamChain_EmptyOverridesAllowsEverything(t *testing.T) {
	t.Parallel()

	chain := policy.TeamChain(policy.TeamOverrides{})
	out, d := chain.Evaluate(req("my key is llmgw_abcdefghijklmnop"))
	assert.Equal(t, policy.ActionAllow, d.Action)
	assert.NotNil(t, out)
}

func TestChain_AllowWhenNoRuleObjects(t *testing.T) {
	t.Parallel()

	chain := policy.NewChain(policy.MaxPromptLength{MaxChars: 1000})
	out, d := chain.Evaluate(req("hello"))
	require.NotNil(t, out)
	assert.Equal(t, policy.ActionAllow, d.Action)
}

func TestChain_DenyShortCircuits(t *testing.T) {
	t.Parallel()

	calls := 0
	spy := spyRule{fn: func(r *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, policy.Decision) {
		calls++
		return r, policy.Decision{Action: policy.ActionAllow}
	}}

	chain := policy.NewChain(policy.MaxPromptLength{MaxChars: 1}, spy)
	out, d := chain.Evaluate(req("way too long for this limit"))

	assert.Equal(t, policy.ActionDeny, d.Action)
	assert.Nil(t, out)
	assert.Zero(t, calls, "rules after a Deny must not run")
}

func TestChain_RedactionsAccumulateAcrossRules(t *testing.T) {
	t.Parallel()

	chain := policy.NewChain(policy.SensitiveDataRedaction{})
	out, d := chain.Evaluate(req("key llmgw_abcdefghijklmnop and card 4111 1111 1111 1111"))

	assert.Equal(t, policy.ActionRedact, d.Action)
	assert.NotContains(t, out.Messages[0].Content, "llmgw_abcdefghijklmnop")
	assert.NotContains(t, out.Messages[0].Content, "4111")
}

func TestDefaultChain_AllowsOrdinaryRequest(t *testing.T) {
	t.Parallel()

	chain := policy.DefaultChain()
	out, d := chain.Evaluate(req("What's a good name for a cat?"))
	require.NotNil(t, out)
	assert.Equal(t, policy.ActionAllow, d.Action)
}

func TestDefaultChain_HonorsCustomBlockedPatterns(t *testing.T) {
	t.Parallel()

	chain := policy.DefaultChain("do-not-say-this")
	_, d := chain.Evaluate(req("please do-not-say-this out loud"))
	assert.Equal(t, policy.ActionDeny, d.Action)
}

type spyRule struct {
	fn func(*openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, policy.Decision)
}

func (s spyRule) Name() string { return "spy" }
func (s spyRule) Evaluate(req *openai.ChatCompletionRequest) (*openai.ChatCompletionRequest, policy.Decision) {
	return s.fn(req)
}
