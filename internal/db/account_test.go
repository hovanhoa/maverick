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
	updated, err := database.UpdateAccount(ctx, account.ID, &newEmail, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newEmail, updated.Email)
	assert.Equal(t, account.Username, updated.Username)

	newUsername := "updateduser"
	updated2, err := database.UpdateAccount(ctx, account.ID, nil, &newUsername)
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

	_, err := database.UpdateAccount(ctx, "any_id", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of email or username")
}

// TestUpdateAccount_NotFound when the id does not exist.
func TestUpdateAccount_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	email := "ghost@example.com"
	_, err := database.UpdateAccount(ctx, "account_missing", &email, nil)
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
