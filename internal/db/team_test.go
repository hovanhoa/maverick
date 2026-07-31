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
	updated, err := database.UpdateAccount(ctx, account.ID, nil, nil, nil, &clear)
	require.NoError(t, err)
	require.Nil(t, updated.TeamID)

	_, err = database.UpdateAccount(ctx, account.ID, nil, nil, &tid, nil)
	require.NoError(t, err)

	both := true
	_, err = database.UpdateAccount(ctx, account.ID, nil, nil, &tid, &both)
	require.Error(t, err)

	_, _ = database.DeleteAccount(ctx, account.ID)
	_, _ = database.DeleteTeam(ctx, team.ID)
}
