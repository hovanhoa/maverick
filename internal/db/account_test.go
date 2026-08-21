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

// TestAccountCRUD exercises CreateAccount, reads, UpdateAccount, and DeleteAccount.
func TestAccountCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account := &model.Account{
		ID:       "account_integration_crud",
		Email:    "crud@example.com",
		Username: "cruduser",
	}

	created, err := database.CreateAccount(ctx, account)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, account.ID, created.ID)
	assert.Equal(t, account.Email, created.Email)
	assert.Equal(t, account.Username, created.Username)
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())

	fetched, err := database.GetAccountByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.Email, fetched.Email)
	assert.Equal(t, created.Username, fetched.Username)

	byPredicate, err := database.GetAccounts(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Eq{"id": account.ID})
	})
	require.NoError(t, err)
	require.Len(t, byPredicate, 1)
	assert.Equal(t, account.ID, byPredicate[0].ID)

	byIDs, err := database.GetAccountsByIDs(ctx, []string{account.ID})
	require.NoError(t, err)
	require.Len(t, byIDs, 1)
	assert.Equal(t, account.ID, byIDs[0].ID)

	newEmail := "updated@example.com"
	updated, err := database.UpdateAccount(ctx, account.ID, &newEmail, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newEmail, updated.Email)
	assert.Equal(t, account.Username, updated.Username)

	newUsername := "updateduser"
	updated2, err := database.UpdateAccount(ctx, account.ID, nil, &newUsername, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated2)
	assert.Equal(t, newEmail, updated2.Email)
	assert.Equal(t, newUsername, updated2.Username)

	deleted, err := database.DeleteAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	gone, err := database.GetAccountByID(ctx, account.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

// TestCreateAccount_GeneratesID when ID is omitted.
func TestCreateAccount_GeneratesID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account := &model.Account{
		Email:    "auto@example.com",
		Username: "autouser",
	}

	created, err := database.CreateAccount(ctx, account)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.True(t, strings.HasPrefix(created.ID, "account_"))

	fetched, err := database.GetAccountByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
}

// TestGetAccountByID_NotFound returns nil account and nil error.
func TestGetAccountByID_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	none, err := database.GetAccountByID(ctx, "account_does_not_exist")
	require.NoError(t, err)
	assert.Nil(t, none)
}

// TestGetAccountsByIDs_EmptyInput returns nil without error.
func TestGetAccountsByIDs_EmptyInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	out, err := database.GetAccountsByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, out)

	out2, err := database.GetAccountsByIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Nil(t, out2)
}

// TestGetAccount_Predicate returns nil when no row matches.
func TestGetAccount_Predicate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	none, err := database.GetAccount(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Eq{"id": "account_no_match"})
	})
	require.NoError(t, err)
	assert.Nil(t, none)
}

// TestUpdateAccount_ValidationError when neither email nor username is set.
func TestUpdateAccount_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	_, err := database.UpdateAccount(ctx, "any_id", nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of email, username, teamId, clearTeamId, or role")
}

// TestUpdateAccount_NotFound when the id does not exist.
func TestUpdateAccount_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	email := "ghost@example.com"
	_, err := database.UpdateAccount(ctx, "account_missing", &email, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account not found")
}

// TestDeleteAccount_NotFound returns false without error.
func TestDeleteAccount_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	ok, err := database.DeleteAccount(ctx, "account_never_created")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestListAccounts returns every account, newest first, and filters by team.
func TestListAccounts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	empty, total, err := database.ListAccounts(ctx, nil, 20, 0)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Zero(t, total)

	team := &model.Team{ID: "team_list_accounts", Name: "Owners"}
	_, err = database.CreateTeam(ctx, team)
	require.NoError(t, err)

	tid := team.ID
	for _, tc := range []struct {
		username string
		teamID   *string
	}{
		{"unassigned", nil},
		{"member_one", &tid},
		{"member_two", &tid},
	} {
		_, err := database.CreateAccount(ctx, &model.Account{
			ID:       "account_list_" + tc.username,
			Email:    tc.username + "@example.com",
			Username: tc.username,
			TeamID:   tc.teamID,
		})
		require.NoError(t, err)
	}

	all, total, err := database.ListAccounts(ctx, nil, 20, 0)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, 3, total)

	// Most recently created first.
	assert.Equal(t, []string{"member_two", "member_one", "unassigned"}, []string{all[0].Username, all[1].Username, all[2].Username})

	byTeam, total, err := database.ListAccounts(ctx, &tid, 20, 0)
	require.NoError(t, err)
	require.Len(t, byTeam, 2)
	assert.Equal(t, 2, total)
	for _, account := range byTeam {
		require.NotNil(t, account.TeamID)
		assert.Equal(t, tid, *account.TeamID)
	}

	other := "team_does_not_exist"
	none, total, err := database.ListAccounts(ctx, &other, 20, 0)
	require.NoError(t, err)
	assert.Empty(t, none)
	assert.Zero(t, total)
}

// TestListAccounts_PaginatesWithinTeamFilter checks that totalCount reflects the
// team filter rather than the whole table, so a paging client sees a consistent
// count and page contents.
func TestListAccounts_PaginatesWithinTeamFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team := &model.Team{ID: "team_page_accounts", Name: "Owners"}
	_, err := database.CreateTeam(ctx, team)
	require.NoError(t, err)
	tid := team.ID

	// Three accounts on the team, two outside it.
	for _, name := range []string{"in_one", "in_two", "in_three"} {
		_, err := database.CreateAccount(ctx, &model.Account{
			ID: "account_page_" + name, Email: name + "@example.com", Username: name, TeamID: &tid,
		})
		require.NoError(t, err)
	}
	for _, name := range []string{"out_one", "out_two"} {
		_, err := database.CreateAccount(ctx, &model.Account{
			ID: "account_page_" + name, Email: name + "@example.com", Username: name,
		})
		require.NoError(t, err)
	}

	first, total, err := database.ListAccounts(ctx, &tid, 2, 0)
	require.NoError(t, err)
	require.Len(t, first, 2)
	assert.Equal(t, 3, total, "totalCount must count only the filtered team")
	assert.Equal(t, []string{"in_three", "in_two"}, []string{first[0].Username, first[1].Username})

	second, total, err := database.ListAccounts(ctx, &tid, 2, 2)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Equal(t, 3, total)
	assert.Equal(t, "in_one", second[0].Username)

	// Unfiltered, the total covers every account.
	_, total, err = database.ListAccounts(ctx, nil, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
}
