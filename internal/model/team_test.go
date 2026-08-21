package model_test

import (
	"testing"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestTeam_IsModelAllowed_EmptyAllowlistAllowsEverything(t *testing.T) {
	t.Parallel()

	team := &model.Team{}
	assert.True(t, team.IsModelAllowed("anthropic", "claude-opus"))
	assert.True(t, team.IsModelAllowed("openai", "gpt-4o"))
}

func TestTeam_IsModelAllowed_ExactMatch(t *testing.T) {
	t.Parallel()

	team := &model.Team{ModelAllowlist: []string{"openai:gpt-4o"}}
	assert.True(t, team.IsModelAllowed("openai", "gpt-4o"))
	assert.False(t, team.IsModelAllowed("openai", "gpt-3.5-turbo"))
	assert.False(t, team.IsModelAllowed("anthropic", "gpt-4o"))
}

func TestTeam_IsModelAllowed_ProviderWildcard(t *testing.T) {
	t.Parallel()

	team := &model.Team{ModelAllowlist: []string{"anthropic:*"}}
	assert.True(t, team.IsModelAllowed("anthropic", "claude-opus"))
	assert.True(t, team.IsModelAllowed("anthropic", "claude-haiku"))
	assert.False(t, team.IsModelAllowed("openai", "gpt-4o"))
}

func TestTeam_IsModelAllowed_MultipleEntries(t *testing.T) {
	t.Parallel()

	team := &model.Team{ModelAllowlist: []string{"anthropic:claude-opus", "openai:*"}}
	assert.True(t, team.IsModelAllowed("anthropic", "claude-opus"))
	assert.False(t, team.IsModelAllowed("anthropic", "claude-haiku"))
	assert.True(t, team.IsModelAllowed("openai", "gpt-4o"))
}

func TestTeam_IsModelAllowed_MalformedEntryIgnored(t *testing.T) {
	t.Parallel()

	team := &model.Team{ModelAllowlist: []string{"not-a-valid-entry"}}
	assert.False(t, team.IsModelAllowed("anthropic", "claude-opus"))
}
