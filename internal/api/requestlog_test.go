package api

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTeamWithTwoMembersRequestLogs creates a team, an OWNER and a MEMBER
// account on it, and records one request_log row per account, for the
// resolver tests below to exercise self/team-admin/global authz branches
// against.
func seedTeamWithTwoMembersRequestLogs(t *testing.T, ctx context.Context, r *Resolver) (team *model.Team, owner, member *model.Account) {
	t.Helper()

	mr := &mutationResolver{r}

	var err error
	team, err = mr.CreateTeam(ctx, "requestlog-resolver-team")
	require.NoError(t, err)

	owner, err = createTestAccount(mr, ctx, "requestlog-owner@example.com", "requestlogowner", &team.ID, nil)
	require.NoError(t, err)
	member, err = createTestAccount(mr, ctx, "requestlog-member@example.com", "requestlogmember", &team.ID, nil)
	require.NoError(t, err)

	require.NoError(t, r.deps.Database.InsertRequestLog(ctx, &model.RequestLog{
		RequestID: "req_owner", AccountID: &owner.ID, TeamID: &team.ID,
		RequestedModel: "anthropic/claude-3-5-sonnet", Status: model.RequestLogStatusSuccess,
		RequestBody: `{"model":"anthropic/claude-3-5-sonnet"}`, LatencyMs: 10,
	}))
	require.NoError(t, r.deps.Database.InsertRequestLog(ctx, &model.RequestLog{
		RequestID: "req_member", AccountID: &member.ID, TeamID: &team.ID,
		RequestedModel: "openai/gpt-4o", Status: model.RequestLogStatusSuccess,
		RequestBody: `{"model":"openai/gpt-4o"}`, LatencyMs: 10,
	}))

	return team, owner, member
}

func TestRequestLogResolver_MyRequestLogs_ScopedToCaller(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, member := seedTeamWithTwoMembersRequestLogs(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	conn, err := qr.MyRequestLogs(memberCtx, nil, nil)
	require.NoError(t, err)
	require.Len(t, conn.Items, 1)
	assert.Equal(t, "req_member", conn.Items[0].RequestID)

	ownerCtx := asPrincipal(ctx, owner.ID, model.RoleOwner, team.ID)
	conn, err = qr.MyRequestLogs(ownerCtx, nil, nil)
	require.NoError(t, err)
	require.Len(t, conn.Items, 1)
	assert.Equal(t, "req_owner", conn.Items[0].RequestID)
}

// TestRequestLogResolver_TeamRequestLogs_MemberDenied asserts the stricter
// tier this query deliberately uses: unlike teamUsage's aggregate numbers,
// a plain MEMBER must not see the team's raw request/response content -
// only an OWNER/ADMIN of that team may.
func TestRequestLogResolver_TeamRequestLogs_MemberDenied(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, _, member := seedTeamWithTwoMembersRequestLogs(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	_, err := qr.TeamRequestLogs(memberCtx, team.ID, nil, nil)
	assert.Error(t, err, "a plain MEMBER must not see the team's request log")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestRequestLogResolver_TeamRequestLogs_AdminAllowed(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, member := seedTeamWithTwoMembersRequestLogs(t, ctx, r)

	ownerCtx := asPrincipal(ctx, owner.ID, model.RoleOwner, team.ID)
	conn, err := qr.TeamRequestLogs(ownerCtx, team.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, conn.Items, 2)
	assert.Equal(t, 2, conn.TotalCount)

	byAccount := map[string]model.RequestLog{}
	for _, entry := range conn.Items {
		byAccount[*entry.AccountID] = entry
	}
	assert.Contains(t, byAccount, owner.ID)
	assert.Contains(t, byAccount, member.ID)
}

// TestRequestLogResolver_TeamRequestLogs_DeniedForAdminOfAnotherTeam mirrors
// the strict-per-team RBAC rule already enforced elsewhere (e.g.
// updateTeamQuota): holding OWNER/ADMIN somewhere is not enough, the
// caller must belong to *this* team.
func TestRequestLogResolver_TeamRequestLogs_DeniedForAdminOfAnotherTeam(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	teamA, err := mr.CreateTeam(ctx, "requestlog-team-a")
	require.NoError(t, err)
	teamB, err := mr.CreateTeam(ctx, "requestlog-team-b")
	require.NoError(t, err)

	caller, err := createTestAccount(mr, ctx, "requestlog-admin-of-a@example.com", "requestlogadminofa", &teamA.ID, nil)
	require.NoError(t, err)
	adminOfACtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, teamA.ID)

	_, err = qr.TeamRequestLogs(adminOfACtx, teamB.ID, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestRequestLogResolver_GlobalRequestLogs_NonAdminDenied(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, _, member := seedTeamWithTwoMembersRequestLogs(t, ctx, r)

	memberCtx := asPrincipal(ctx, member.ID, model.RoleMember, team.ID)
	_, err := qr.GlobalRequestLogs(memberCtx, nil, nil)
	assert.Error(t, err, "a plain MEMBER must not see the platform-wide request log")
}

func TestRequestLogResolver_GlobalRequestLogs_AdminAllowedAcrossTeams(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}
	team, owner, _ := seedTeamWithTwoMembersRequestLogs(t, ctx, r)

	// owner is an OWNER of `team`, but globalRequestLogs must not be scoped
	// to that team - requireRole checks role only, so an OWNER anywhere
	// sees platform-wide entries across every team.
	ownerCtx := asPrincipal(ctx, owner.ID, model.RoleOwner, team.ID)

	conn, err := qr.GlobalRequestLogs(ownerCtx, nil, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, conn.TotalCount, 2)
}
