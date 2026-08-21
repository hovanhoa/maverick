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

	fetched, err := qr.Team(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)

	updated, err := mr.UpdateTeam(ctx, created.ID, "resolver-team-renamed")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "resolver-team-renamed", updated.Name)

	deleted, err := mr.DeleteTeam(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	gone, err := qr.Team(ctx, created.ID)
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

	_, _ = mr.DeleteTeam(ctx, created.ID)
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

	_, _ = mr.DeleteAccount(ctx, acc.ID)
	_, _ = mr.DeleteTeam(ctx, team.ID)
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

	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember)

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

	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin)

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
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember)

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
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember)

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
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember)

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
	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin)

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

	// No allowlist configured yet: everything is allowed.
	allowed, err := qr.IsModelAllowed(ctx, team.ID, "anthropic", "claude-opus")
	require.NoError(t, err)
	assert.True(t, allowed)

	_, err = mr.UpdateTeamModelAllowlist(ctx, team.ID, []string{"anthropic:*"})
	require.NoError(t, err)

	allowed, err = qr.IsModelAllowed(ctx, team.ID, "anthropic", "claude-opus")
	require.NoError(t, err)
	assert.True(t, allowed)

	denied, err := qr.IsModelAllowed(ctx, team.ID, "openai", "gpt-4o")
	require.NoError(t, err)
	assert.False(t, denied)
}

func TestQueryResolver_IsModelAllowed_TeamNotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}

	_, err := qr.IsModelAllowed(ctx, "team_missing", "anthropic", "claude-opus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team not found")
}
