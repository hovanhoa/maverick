package db_test

import (
	"context"
	"strings"
	"testing"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team := &model.Team{
		ID:   "team_integration_crud",
		Name: "Platform",
	}

	created, err := database.CreateTeam(ctx, team)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, team.ID, created.ID)
	assert.Equal(t, team.Name, created.Name)

	fetched, err := database.GetTeamByID(ctx, team.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.Name, fetched.Name)

	byPredicate, err := database.GetTeams(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Eq{"id": team.ID})
	})
	require.NoError(t, err)
	require.Len(t, byPredicate, 1)

	updated, err := database.UpdateTeam(ctx, team.ID, "Platform II")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Platform II", updated.Name)

	deleted, err := database.DeleteTeam(ctx, team.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	gone, err := database.GetTeamByID(ctx, team.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestCreateTeam_GeneratesID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team := &model.Team{Name: "SRE"}

	created, err := database.CreateTeam(ctx, team)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.True(t, strings.HasPrefix(created.ID, "team_"))
}

func TestListTeams(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	empty, total, err := database.ListTeams(ctx, 20, 0)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Zero(t, total)

	for _, name := range []string{"first", "second", "third"} {
		_, err := database.CreateTeam(ctx, &model.Team{ID: "team_list_" + name, Name: name})
		require.NoError(t, err)
	}

	teams, total, err := database.ListTeams(ctx, 20, 0)
	require.NoError(t, err)
	require.Len(t, teams, 3)
	assert.Equal(t, 3, total)

	// Most recently created first.
	assert.Equal(t, []string{"third", "second", "first"}, []string{teams[0].Name, teams[1].Name, teams[2].Name})
}

func TestListTeams_Paginates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	for _, name := range []string{"first", "second", "third", "fourth", "fifth"} {
		_, err := database.CreateTeam(ctx, &model.Team{ID: "team_page_" + name, Name: name})
		require.NoError(t, err)
	}

	// Walk the whole set two at a time; totalCount stays constant across pages.
	var seen []string
	for offset := 0; offset < 6; offset += 2 {
		page, total, err := database.ListTeams(ctx, 2, offset)
		require.NoError(t, err)
		assert.Equal(t, 5, total)

		for _, team := range page {
			seen = append(seen, team.Name)
		}
	}

	assert.Equal(t, []string{"fifth", "fourth", "third", "second", "first"}, seen)

	// An offset past the end yields no rows but still reports the true total.
	none, total, err := database.ListTeams(ctx, 2, 99)
	require.NoError(t, err)
	assert.Empty(t, none)
	assert.Equal(t, 5, total)
}

func TestAccountWithTeamJSONB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team := &model.Team{ID: "team_for_account", Name: "Owners"}
	_, err := database.CreateTeam(ctx, team)
	require.NoError(t, err)

	tid := team.ID
	account := &model.Account{
		ID:       "account_with_team",
		Email:    "member@example.com",
		Username: "member",
		TeamID:   &tid,
	}
	created, err := database.CreateAccount(ctx, account)
	require.NoError(t, err)
	require.NotNil(t, created.TeamID)
	assert.Equal(t, team.ID, *created.TeamID)

	fetched, err := database.GetAccountByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.NotNil(t, fetched.TeamID)
	assert.Equal(t, team.ID, *fetched.TeamID)

	clear := true
	updated, err := database.UpdateAccount(ctx, account.ID, nil, nil, nil, nil, &clear, nil)
	require.NoError(t, err)
	require.Nil(t, updated.TeamID)

	_, err = database.UpdateAccount(ctx, account.ID, nil, nil, nil, &tid, nil, nil)
	require.NoError(t, err)

	both := true
	_, err = database.UpdateAccount(ctx, account.ID, nil, nil, nil, &tid, &both, nil)
	require.Error(t, err)

	_, _ = database.DeleteAccount(ctx, account.ID)
	_, _ = database.DeleteTeam(ctx, team.ID)
}

func TestUpdateTeamModelAllowlist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Allowlist team"})
	require.NoError(t, err)
	assert.Empty(t, team.ModelAllowlist, "new teams start with no restriction configured")

	updated, err := database.UpdateTeamModelAllowlist(ctx, team.ID, []string{"anthropic:*", "openai:gpt-4o"})
	require.NoError(t, err)
	assert.Equal(t, []string{"anthropic:*", "openai:gpt-4o"}, updated.ModelAllowlist)

	fetched, err := database.GetTeamByID(ctx, team.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, []string{"anthropic:*", "openai:gpt-4o"}, fetched.ModelAllowlist)

	// Replacing with an empty list clears the restriction wholesale.
	cleared, err := database.UpdateTeamModelAllowlist(ctx, team.ID, []string{})
	require.NoError(t, err)
	assert.Empty(t, cleared.ModelAllowlist)
}

func TestUpdateTeamModelAllowlist_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	_, err := database.UpdateTeamModelAllowlist(ctx, "team_missing", []string{"anthropic:*"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team not found")
}

func TestUpdateTeamPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Policy team"})
	require.NoError(t, err)
	assert.Empty(t, team.Policy.BlockedPatterns, "new teams start with no policy overrides configured")
	assert.False(t, team.Policy.DenyOnSensitiveData)

	updated, err := database.UpdateTeamPolicy(ctx, team.ID, []string{"company-secret-project"}, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"company-secret-project"}, updated.Policy.BlockedPatterns)
	assert.True(t, updated.Policy.DenyOnSensitiveData)

	fetched, err := database.GetTeamByID(ctx, team.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, []string{"company-secret-project"}, fetched.Policy.BlockedPatterns)
	assert.True(t, fetched.Policy.DenyOnSensitiveData)

	// Replacing wholesale with empty/false clears the overrides.
	cleared, err := database.UpdateTeamPolicy(ctx, team.ID, []string{}, false)
	require.NoError(t, err)
	assert.Empty(t, cleared.Policy.BlockedPatterns)
	assert.False(t, cleared.Policy.DenyOnSensitiveData)
}

func TestUpdateTeamPolicy_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	_, err := database.UpdateTeamPolicy(ctx, "team_missing", []string{"x"}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team not found")
}

func TestUpdateTeamQuota(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Quota team"})
	require.NoError(t, err)
	assert.Nil(t, team.MonthlyTokenBudget, "new teams start unlimited")

	budget := 1_000_000
	updated, err := database.UpdateTeamQuota(ctx, team.ID, &budget, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.MonthlyTokenBudget)
	assert.Equal(t, budget, *updated.MonthlyTokenBudget)

	fetched, err := database.GetTeamByID(ctx, team.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.MonthlyTokenBudget)
	assert.Equal(t, budget, *fetched.MonthlyTokenBudget)

	clear := true
	cleared, err := database.UpdateTeamQuota(ctx, team.ID, nil, &clear)
	require.NoError(t, err)
	assert.Nil(t, cleared.MonthlyTokenBudget)
}

func TestUpdateTeamQuota_RequiresOneField(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Quota team 2"})
	require.NoError(t, err)

	_, err = database.UpdateTeamQuota(ctx, team.ID, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of")
}

func TestUpdateTeamQuota_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	budget := 100
	_, err := database.UpdateTeamQuota(ctx, "team_missing", &budget, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team not found")
}
