package api

import (
	"strings"
	"testing"

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

	acc, err := mr.CreateAccount(ctx, "withteam@example.com", "withteam", &team.ID)
	require.NoError(t, err)
	require.NotNil(t, acc.TeamID)
	assert.Equal(t, team.ID, *acc.TeamID)

	_, _ = mr.DeleteAccount(ctx, acc.ID)
	_, _ = mr.DeleteTeam(ctx, team.ID)
}
