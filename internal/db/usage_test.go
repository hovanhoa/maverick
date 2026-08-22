package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordUsageEvent_AndSumTeamUsage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Usage team"})
	require.NoError(t, err)
	account, err := database.CreateAccount(ctx, &model.Account{Email: "usage@example.com", Username: "usageuser", TeamID: &team.ID})
	require.NoError(t, err)

	since := time.Now().Add(-time.Hour)

	require.NoError(t, database.RecordUsageEvent(ctx, &model.UsageEvent{
		RequestID: "req_1", AccountID: account.ID, TeamID: &team.ID,
		Provider: "anthropic", Model: "claude-3-5-sonnet",
		PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, CostUSD: 0.0025,
	}))
	require.NoError(t, database.RecordUsageEvent(ctx, &model.UsageEvent{
		RequestID: "req_2", AccountID: account.ID, TeamID: &team.ID,
		Provider: "openai", Model: "gpt-4o",
		PromptTokens: 200, CompletionTokens: 100, TotalTokens: 300, CostUSD: 0.0015,
	}))

	summary, err := database.SumTeamUsage(ctx, team.ID, since)
	require.NoError(t, err)
	assert.Equal(t, 2, summary.RequestCount)
	assert.Equal(t, 300, summary.PromptTokens)
	assert.Equal(t, 150, summary.CompletionTokens)
	assert.Equal(t, 450, summary.TotalTokens)
	assert.InDelta(t, 0.004, summary.CostUSD, 0.0001)
}

func TestSumTeamUsage_ExcludesEventsBeforeSince(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Usage window team"})
	require.NoError(t, err)
	account, err := database.CreateAccount(ctx, &model.Account{Email: "usage2@example.com", Username: "usageuser2", TeamID: &team.ID})
	require.NoError(t, err)

	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, database.RecordUsageEvent(ctx, &model.UsageEvent{
		RequestID: "req_old", AccountID: account.ID, TeamID: &team.ID,
		Provider: "anthropic", Model: "claude-3-5-sonnet",
		PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20, CostUSD: 0.001,
		CreatedAt: old,
	}))

	summary, err := database.SumTeamUsage(ctx, team.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, summary.RequestCount)
}

// TestDeleteAccount_WithRecordedUsageEvent verifies that deleting an
// account which has already made a metered proxy call still succeeds - the
// usage_event row is a durable billing/audit trail, so its account_id FK
// must be ON DELETE SET NULL rather than blocking the delete (or cascading
// and silently erasing billing history).
func TestDeleteAccount_WithRecordedUsageEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "deletable@example.com", Username: "deletable"})
	require.NoError(t, err)

	require.NoError(t, database.RecordUsageEvent(ctx, &model.UsageEvent{
		RequestID: "req_del", AccountID: account.ID,
		Provider: "anthropic", Model: "claude-3-5-sonnet",
		PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, CostUSD: 0.001,
	}))

	ok, err := database.DeleteAccount(ctx, account.ID)
	require.NoError(t, err, "deleting an account must not fail just because it has recorded usage")
	assert.True(t, ok)
}

// TestDeleteTeam_WithRecordedUsageEvent mirrors
// TestDeleteAccount_WithRecordedUsageEvent for the team_id FK.
func TestDeleteTeam_WithRecordedUsageEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Deletable team"})
	require.NoError(t, err)
	account, err := database.CreateAccount(ctx, &model.Account{Email: "deletable2@example.com", Username: "deletable2", TeamID: &team.ID})
	require.NoError(t, err)

	require.NoError(t, database.RecordUsageEvent(ctx, &model.UsageEvent{
		RequestID: "req_del_team", AccountID: account.ID, TeamID: &team.ID,
		Provider: "anthropic", Model: "claude-3-5-sonnet",
		PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, CostUSD: 0.001,
	}))

	ok, err := database.DeleteTeam(ctx, team.ID)
	require.NoError(t, err, "deleting a team must not fail just because it has recorded usage")
	assert.True(t, ok)
}

func TestSumTeamUsage_NoEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Empty usage team"})
	require.NoError(t, err)

	summary, err := database.SumTeamUsage(ctx, team.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, summary.RequestCount)
	assert.Zero(t, summary.CostUSD)
}
