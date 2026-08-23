package db_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestInsertRequestLog_AndListRequestLogs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Request log team"})
	require.NoError(t, err)
	account, err := database.CreateAccount(ctx, &model.Account{Email: "reqlog@example.com", Username: "reqloguser", TeamID: &team.ID})
	require.NoError(t, err)

	require.NoError(t, database.InsertRequestLog(ctx, &model.RequestLog{
		RequestID: "req_success", AccountID: account.ID, TeamID: &team.ID,
		Provider: ptr("anthropic"), Model: ptr("claude-3-5-sonnet"), RequestedModel: "anthropic/claude-3-5-sonnet",
		Status: model.RequestLogStatusSuccess, RequestBody: `{"model":"anthropic/claude-3-5-sonnet"}`,
		ResponseBody: ptr(`{"id":"resp_1"}`), PromptTokens: ptr(10), CompletionTokens: ptr(5), TotalTokens: ptr(15),
		CostUsd: ptr(0.001), LatencyMs: 120,
	}))
	require.NoError(t, database.InsertRequestLog(ctx, &model.RequestLog{
		RequestID: "req_denied", AccountID: account.ID, TeamID: &team.ID,
		RequestedModel: "bogus", Status: model.RequestLogStatusError,
		ErrorKind: ptr("invalid_request"), ErrorMessage: ptr(`model must be formatted as "provider/model"`),
		RequestBody: `{"model":"bogus"}`, LatencyMs: 1,
	}))

	logs, total, err := database.ListRequestLogs(ctx, db.RequestLogFilter{TeamID: &team.ID}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, logs, 2)

	byRequestID := map[string]model.RequestLog{}
	for _, l := range logs {
		byRequestID[l.RequestID] = l
	}

	success := byRequestID["req_success"]
	assert.Equal(t, model.RequestLogStatusSuccess, success.Status)
	require.NotNil(t, success.Provider)
	assert.Equal(t, "anthropic", *success.Provider)
	require.NotNil(t, success.ResponseBody)
	assert.JSONEq(t, `{"id":"resp_1"}`, *success.ResponseBody, "JSONB round-trips through Postgres's own (re-)serialization, not byte-for-byte")
	require.NotNil(t, success.TotalTokens)
	assert.Equal(t, 15, *success.TotalTokens)

	denied := byRequestID["req_denied"]
	assert.Equal(t, model.RequestLogStatusError, denied.Status)
	assert.Nil(t, denied.Provider, "a call that failed before resolving a provider must leave provider/model null")
	assert.Nil(t, denied.ResponseBody)
	require.NotNil(t, denied.ErrorKind)
	assert.Equal(t, "invalid_request", *denied.ErrorKind)
}

func TestListRequestLogs_ScopedByAccountAndPaginated(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	accountA, err := database.CreateAccount(ctx, &model.Account{Email: "reqloga@example.com", Username: "reqlogusera"})
	require.NoError(t, err)
	accountB, err := database.CreateAccount(ctx, &model.Account{Email: "reqlogb@example.com", Username: "reqloguserb"})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		require.NoError(t, database.InsertRequestLog(ctx, &model.RequestLog{
			RequestID: "req_a", AccountID: accountA.ID, RequestedModel: "openai/gpt-4o",
			Status: model.RequestLogStatusSuccess, RequestBody: "{}", LatencyMs: 1,
		}))
	}
	require.NoError(t, database.InsertRequestLog(ctx, &model.RequestLog{
		RequestID: "req_b", AccountID: accountB.ID, RequestedModel: "openai/gpt-4o",
		Status: model.RequestLogStatusSuccess, RequestBody: "{}", LatencyMs: 1,
	}))

	logsA, totalA, err := database.ListRequestLogs(ctx, db.RequestLogFilter{AccountID: &accountA.ID}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, totalA)
	assert.Len(t, logsA, 3)

	page, totalA2, err := database.ListRequestLogs(ctx, db.RequestLogFilter{AccountID: &accountA.ID}, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, totalA2)
	assert.Len(t, page, 2, "limit must cap the page even though more rows match")

	rest, _, err := database.ListRequestLogs(ctx, db.RequestLogFilter{AccountID: &accountA.ID}, 2, 2)
	require.NoError(t, err)
	assert.Len(t, rest, 1, "offset must skip the rows already returned by the first page")
}

// TestDeleteAccount_WithRecordedRequestLog mirrors
// TestDeleteAccount_WithRecordedUsageEvent: a request_log row is a durable
// audit trail, so its account_id FK must be ON DELETE SET NULL rather than
// blocking the delete.
func TestDeleteAccount_WithRecordedRequestLog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "reqlogdel@example.com", Username: "reqlogdel"})
	require.NoError(t, err)

	require.NoError(t, database.InsertRequestLog(ctx, &model.RequestLog{
		RequestID: "req_del", AccountID: account.ID, RequestedModel: "openai/gpt-4o",
		Status: model.RequestLogStatusSuccess, RequestBody: "{}", LatencyMs: 1,
	}))

	ok, err := database.DeleteAccount(ctx, account.ID)
	require.NoError(t, err, "deleting an account must not fail just because it has a recorded request log")
	assert.True(t, ok)
}

// TestDeleteTeam_WithRecordedRequestLog mirrors
// TestDeleteAccount_WithRecordedRequestLog for the team_id FK.
func TestDeleteTeam_WithRecordedRequestLog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Deletable req log team"})
	require.NoError(t, err)
	account, err := database.CreateAccount(ctx, &model.Account{Email: "reqlogdelteam@example.com", Username: "reqlogdelteam", TeamID: &team.ID})
	require.NoError(t, err)

	require.NoError(t, database.InsertRequestLog(ctx, &model.RequestLog{
		RequestID: "req_del_team", AccountID: account.ID, TeamID: &team.ID, RequestedModel: "openai/gpt-4o",
		Status: model.RequestLogStatusSuccess, RequestBody: "{}", LatencyMs: 1,
	}))

	ok, err := database.DeleteTeam(ctx, team.ID)
	require.NoError(t, err, "deleting a team must not fail just because it has a recorded request log")
	assert.True(t, ok)
}
