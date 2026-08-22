package api

import (
	"strings"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamResolver_CRUD(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	created, err := mr.CreateTeam(ctx, "resolver-team")
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "resolver-team", created.Name)

	// createTeam doesn't update the RBAC-exempt test principal's OrgID (it
	// only writes the DB row for real, account-backed callers), so scope a
	// fresh context to the team just created for the rest of the lifecycle.
	teamCtx := asPrincipal(ctx, "account_test_caller", model.RoleOwner, created.ID)

	fetched, err := qr.Team(teamCtx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)

	updated, err := mr.UpdateTeam(teamCtx, created.ID, "resolver-team-renamed")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "resolver-team-renamed", updated.Name)

	deleted, err := mr.DeleteTeam(teamCtx, created.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	gone, err := qr.Team(teamCtx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestTeamResolver_CreateTeam_GeneratesID(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	created, err := mr.CreateTeam(ctx, "id-gen")
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.True(t, strings.HasPrefix(created.ID, "team_"))

	teamCtx := asPrincipal(ctx, "account_test_caller", model.RoleOwner, created.ID)
	_, _ = mr.DeleteTeam(teamCtx, created.ID)
}

func TestAccountResolver_CreateWithTeam(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "home")
	require.NoError(t, err)

	acc, err := mr.CreateAccount(ctx, "withteam@example.com", "withteam", &team.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, acc.TeamID)
	assert.Equal(t, team.ID, *acc.TeamID)

	teamCtx := asPrincipal(ctx, "account_test_caller", model.RoleOwner, team.ID)
	_, _ = mr.DeleteAccount(teamCtx, acc.ID)
	_, _ = mr.DeleteTeam(teamCtx, team.ID)
}

func TestTeamResolver_Teams(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	empty, err := qr.Teams(ctx, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	assert.Empty(t, empty.Items)
	assert.Zero(t, empty.TotalCount)
	assert.False(t, empty.HasNextPage)

	for _, name := range []string{"alpha", "beta"} {
		_, err := mr.CreateTeam(ctx, name)
		require.NoError(t, err)
	}

	teams, err := qr.Teams(ctx, nil, nil)
	require.NoError(t, err)
	require.Len(t, teams.Items, 2)
	assert.Equal(t, 2, teams.TotalCount)
	assert.False(t, teams.HasNextPage)
	assert.Equal(t, "beta", teams.Items[0].Name)
	assert.Equal(t, "alpha", teams.Items[1].Name)
}

func TestTeamResolver_Teams_Pagination(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		_, err := mr.CreateTeam(ctx, name)
		require.NoError(t, err)
	}

	limit, offset := 2, 0
	first, err := qr.Teams(ctx, &limit, &offset)
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	assert.Equal(t, 3, first.TotalCount)
	assert.True(t, first.HasNextPage, "one team remains after this page")
	assert.Equal(t, []string{"gamma", "beta"}, []string{first.Items[0].Name, first.Items[1].Name})

	offset = 2
	last, err := qr.Teams(ctx, &limit, &offset)
	require.NoError(t, err)
	require.Len(t, last.Items, 1)
	assert.Equal(t, 3, last.TotalCount)
	assert.False(t, last.HasNextPage, "final page must not advertise another")
	assert.Equal(t, "alpha", last.Items[0].Name)

	// Past the end: empty page, true total, no next page.
	offset = 99
	beyond, err := qr.Teams(ctx, &limit, &offset)
	require.NoError(t, err)
	assert.Empty(t, beyond.Items)
	assert.Equal(t, 3, beyond.TotalCount)
	assert.False(t, beyond.HasNextPage)
}

func TestTeamResolver_Teams_RejectsBadBounds(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}

	zero := 0
	_, err := qr.Teams(ctx, &zero, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit must be greater than 0")

	negative := -1
	_, err = qr.Teams(ctx, nil, &negative)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "offset must not be negative")
}

// TestTeamResolver_CreateTeam_DeniedForMember asserts the RBAC rule that
// only OWNER/ADMIN callers may create teams.
func TestTeamResolver_CreateTeam_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	caller, err := mr.CreateAccount(ctx, "member-team-creator@example.com", "memberteamcreator", nil, nil)
	require.NoError(t, err)

	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	_, err = mr.CreateTeam(memberCtx, "should-not-exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

// TestTeamResolver_CreateTeam_AutoAssignsOwner asserts that the creating
// account is auto-assigned OWNER of the team it just created.
func TestTeamResolver_CreateTeam_AutoAssignsOwner(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	caller, err := mr.CreateAccount(ctx, "admin-team-creator@example.com", "adminteamcreator", nil, nil)
	require.NoError(t, err)

	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, "")

	team, err := mr.CreateTeam(adminCtx, "auto-owner-team")
	require.NoError(t, err)

	updatedCaller, err := (&queryResolver{r}).Account(ctx, caller.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedCaller.TeamID)
	assert.Equal(t, team.ID, *updatedCaller.TeamID)
	assert.Equal(t, model.RoleOwner, updatedCaller.Role)
}

// TestTeamResolver_UpdateTeam_DeniedForMember and
// TestTeamResolver_DeleteTeam_DeniedForMember assert the same RBAC rule for
// updateTeam and deleteTeam.
func TestTeamResolver_UpdateTeam_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "update-rbac-team")
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "member-team-updater@example.com", "memberteamupdater", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	_, err = mr.UpdateTeam(memberCtx, team.ID, "renamed")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestTeamResolver_DeleteTeam_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "delete-rbac-team")
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "member-team-deleter@example.com", "memberteamdeleter", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	_, err = mr.DeleteTeam(memberCtx, team.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestTeamResolver_UpdateTeamModelAllowlist_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "allowlist-rbac-team")
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "member-allowlist@example.com", "memberallowlist", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	_, err = mr.UpdateTeamModelAllowlist(memberCtx, team.ID, []string{"anthropic:*"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestTeamResolver_UpdateTeamModelAllowlist_AllowedForAdmin(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "allowlist-admin-team")
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "admin-allowlist@example.com", "adminallowlist", nil, nil)
	require.NoError(t, err)
	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, team.ID)

	updated, err := mr.UpdateTeamModelAllowlist(adminCtx, team.ID, []string{"anthropic:*", "openai:gpt-4o"})
	require.NoError(t, err)
	assert.Equal(t, []string{"anthropic:*", "openai:gpt-4o"}, updated.ModelAllowlist)
}

func TestQueryResolver_IsModelAllowed(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	team, err := mr.CreateTeam(ctx, "is-model-allowed-team")
	require.NoError(t, err)
	teamCtx := asPrincipal(ctx, "account_test_caller", model.RoleOwner, team.ID)

	// No allowlist configured yet: everything is allowed.
	allowed, err := qr.IsModelAllowed(teamCtx, team.ID, "anthropic", "claude-opus")
	require.NoError(t, err)
	assert.True(t, allowed)

	_, err = mr.UpdateTeamModelAllowlist(teamCtx, team.ID, []string{"anthropic:*"})
	require.NoError(t, err)

	allowed, err = qr.IsModelAllowed(teamCtx, team.ID, "anthropic", "claude-opus")
	require.NoError(t, err)
	assert.True(t, allowed)

	denied, err := qr.IsModelAllowed(teamCtx, team.ID, "openai", "gpt-4o")
	require.NoError(t, err)
	assert.False(t, denied)
}

func TestQueryResolver_IsModelAllowed_TeamNotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}

	// Scoped to "team_missing" itself so the membership check passes and
	// the underlying "team not found" lookup is what's actually exercised.
	missingTeamCtx := asPrincipal(ctx, "account_test_caller", model.RoleOwner, "team_missing")

	_, err := qr.IsModelAllowed(missingTeamCtx, "team_missing", "anthropic", "claude-opus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team not found")
}

// TestQueryResolver_IsModelAllowed_DeniedForNonMember asserts the RBAC rule
// this phase adds: a principal who doesn't belong to the team can't even
// read its allowlist status, regardless of role.
func TestQueryResolver_IsModelAllowed_DeniedForNonMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	team, err := mr.CreateTeam(ctx, "is-model-allowed-outsider-team")
	require.NoError(t, err)

	outsider, err := mr.CreateAccount(ctx, "outsider-model-check@example.com", "outsidermodelcheck", nil, nil)
	require.NoError(t, err)
	outsiderCtx := asPrincipal(ctx, outsider.ID, model.RoleOwner, "team_someone_elses")

	_, err = qr.IsModelAllowed(outsiderCtx, team.ID, "anthropic", "claude-opus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestTeamResolver_UpdateTeamQuota_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "quota-rbac-team")
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "member-quota@example.com", "memberquota", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	budget := 1000
	_, err = mr.UpdateTeamQuota(memberCtx, team.ID, &budget, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestTeamResolver_UpdateTeamQuota_AllowedForAdmin(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "quota-admin-team")
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "admin-quota@example.com", "adminquota", nil, nil)
	require.NoError(t, err)
	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, team.ID)

	budget := 5000
	updated, err := mr.UpdateTeamQuota(adminCtx, team.ID, &budget, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.MonthlyTokenBudget)
	assert.Equal(t, budget, *updated.MonthlyTokenBudget)
}

// TestTeamResolver_UpdateTeamQuota_DeniedForAdminOfAnotherTeam is the core
// assertion of the strict-per-team RBAC redesign: holding OWNER/ADMIN
// somewhere is not enough - the caller must belong to *this* team.
func TestTeamResolver_UpdateTeamQuota_DeniedForAdminOfAnotherTeam(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	teamA, err := mr.CreateTeam(ctx, "quota-team-a")
	require.NoError(t, err)
	teamB, err := mr.CreateTeam(ctx, "quota-team-b")
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "admin-of-a@example.com", "adminofa", &teamA.ID, nil)
	require.NoError(t, err)
	adminOfACtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, teamA.ID)

	budget := 5000
	_, err = mr.UpdateTeamQuota(adminOfACtx, teamB.ID, &budget, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestQueryResolver_TeamUsage_EmptyByDefault(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	team, err := mr.CreateTeam(ctx, "usage-query-team")
	require.NoError(t, err)
	teamCtx := asPrincipal(ctx, "account_test_caller", model.RoleOwner, team.ID)

	summary, err := qr.TeamUsage(teamCtx, team.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.RequestCount)
	assert.Zero(t, summary.CostUsd)
}

// TestQueryResolver_TeamUsage_DeniedForNonMember asserts that usage/cost
// data - more sensitive than a team's name or allowlist - is only visible
// to that team's own members.
func TestQueryResolver_TeamUsage_DeniedForNonMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	team, err := mr.CreateTeam(ctx, "usage-query-outsider-team")
	require.NoError(t, err)

	outsider, err := mr.CreateAccount(ctx, "outsider-usage@example.com", "outsiderusage", nil, nil)
	require.NoError(t, err)
	outsiderCtx := asPrincipal(ctx, outsider.ID, model.RoleOwner, "team_someone_elses")

	_, err = qr.TeamUsage(outsiderCtx, team.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}
