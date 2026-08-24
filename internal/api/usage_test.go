package api

import (
	"context"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTeamWithTwoMembersUsage creates a team, an OWNER and a MEMBER account
// on it, and records one usage_event for each, for the resolver tests
// below to exercise self/team-admin/global authz branches against.
func seedTeamWithTwoMembersUsage(t *testing.T, ctx context.Context, r *Resolver) (team *model.Team, owner, member *model.Account) {
	t.Helper()

	mr := &mutationResolver{r}

	var err error
	team, err = mr.CreateTeam(ctx, "usage-resolver-team")
	require.NoError(t, err)

	owner, err = createTestAccount(mr, ctx, "usage-owner@example.com", "usageowner", &team.ID, nil)
	require.NoError(t, err)
	member, err = createTestAccount(mr, ctx, "usage-member@example.com", "usagemember", &team.ID, nil)
	require.NoError(t, err)

	require.NoError(t, r.deps.Database.RecordUsageEvent(ctx, &model.UsageEvent{
		RequestID: "req_owner", AccountID: owner.ID, TeamID: &team.ID,
		Provider: "anthropic", Model: "claude-3-5-sonnet",
		PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, CostUSD: 1.00,
	}))
	require.NoError(t, r.deps.Database.RecordUsageEvent(ctx, &model.UsageEvent{
		RequestID: "req_member", AccountID: member.ID, TeamID: &team.ID,
		Provider: "openai", Model: "gpt-4o",
		PromptTokens: 200, CompletionTokens: 100, TotalTokens: 300, CostUSD: 2.00,
	}))

	return team, owner, member
}

func TestUsageResolver_MyUsage_ReturnsCallersOwnUsage(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, member := seedTeamWithTwoMembersUsage(t, ctx, r)
	_ = team

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	summary, err := qr.MyUsage(memberCtx, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.RequestCount)
	assert.InDelta(t, 2.00, summary.CostUsd, 0.0001)

	ownerCtx := asPrincipal(ctx, owner.ID, model.RoleOwner, team.ID)
	summary, err = qr.MyUsage(ownerCtx, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.RequestCount)
	assert.InDelta(t, 1.00, summary.CostUsd, 0.0001)
}

func TestUsageResolver_AccountUsage_SelfAllowed(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, _, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	summary, err := qr.AccountUsage(memberCtx, member.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.RequestCount)
}

func TestUsageResolver_AccountUsage_OtherMemberDenied(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	_, err := qr.AccountUsage(memberCtx, owner.ID, nil)
	assert.Error(t, err, "a plain MEMBER must not be able to see another account's usage")
}

func TestUsageResolver_AccountUsage_TeamAdminAllowedForOtherMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	ownerCtx := asPrincipal(ctx, owner.ID, model.RoleOwner, team.ID)
	summary, err := qr.AccountUsage(ownerCtx, member.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.RequestCount)
	assert.InDelta(t, 2.00, summary.CostUsd, 0.0001)
}

func TestUsageResolver_TeamUsageByAccount_MemberDenied(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, _, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	_, err := qr.TeamUsageByAccount(memberCtx, team.ID, nil)
	assert.Error(t, err, "a plain MEMBER must not see the per-member usage breakdown")
}

func TestUsageResolver_TeamUsageByAccount_AdminAllowed(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	ownerCtx := asPrincipal(ctx, owner.ID, model.RoleOwner, team.ID)
	rows, err := qr.TeamUsageByAccount(ownerCtx, team.ID, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byAccount := map[string]model.AccountUsage{}
	for _, row := range rows {
		byAccount[*row.AccountID] = row
	}
	assert.InDelta(t, 1.00, byAccount[owner.ID].CostUsd, 0.0001)
	assert.InDelta(t, 2.00, byAccount[member.ID].CostUsd, 0.0001)
}

func TestUsageResolver_TeamUsageByModel_OpenToAnyMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, _, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	rows, err := qr.TeamUsageByModel(memberCtx, team.ID, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestUsageResolver_TeamUsageDaily_OpenToAnyMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, _, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	since := time.Now().Add(-24 * time.Hour)
	until := time.Now().Add(time.Hour)
	rows, err := qr.TeamUsageDaily(memberCtx, team.ID, &since, &until)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].RequestCount)
}

func TestUsageResolver_GlobalUsage_NonAdminDenied(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, _, member := seedTeamWithTwoMembersUsage(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	_, err := qr.GlobalUsage(memberCtx, nil)
	assert.Error(t, err, "a plain MEMBER must not see platform-wide usage")

	_, err = qr.GlobalUsageByTeam(memberCtx, nil)
	assert.Error(t, err, "a plain MEMBER must not see the platform-wide usage-by-team breakdown")
}

func TestUsageResolver_GlobalUsage_AdminAllowedAcrossTeams(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, _ := seedTeamWithTwoMembersUsage(t, ctx, r)

	// owner is an OWNER of `team`, but globalUsage/globalUsageByTeam must
	// not be scoped to that team - requireRole checks role only, so an
	// OWNER anywhere sees platform-wide totals across every team.
	ownerCtx := asPrincipal(ctx, owner.ID, model.RoleOwner, team.ID)

	summary, err := qr.GlobalUsage(ownerCtx, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, summary.RequestCount, 2)

	byTeam, err := qr.GlobalUsageByTeam(ownerCtx, nil)
	require.NoError(t, err)

	var found bool
	for _, row := range byTeam {
		if row.TeamID == team.ID {
			found = true
			assert.Equal(t, 2, row.RequestCount)
		}
	}
	assert.True(t, found, "the seeded team must appear in the global-by-team breakdown")
}
