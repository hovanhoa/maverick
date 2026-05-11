package api

import (
	"context"
	"strings"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testResolver(t *testing.T) (*Resolver, context.Context) {
	t.Helper()
	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	return &Resolver{deps: Dependencies{Database: database}}, ctx
}

// TestAccountResolver_CRUD exercises CreateAccount, Account, UpdateAccount, and DeleteAccount.
func TestAccountResolver_CRUD(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	created, err := mr.CreateAccount(ctx, "api-crud@example.com", "apicrud")
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "api-crud@example.com", created.Email)
	assert.Equal(t, "apicrud", created.Username)
	assert.False(t, created.CreatedAt.IsZero())

	fetched, err := qr.Account(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Email, fetched.Email)

	newEmail := "api-updated@example.com"
	updated, err := mr.UpdateAccount(ctx, created.ID, &newEmail, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newEmail, updated.Email)
	assert.Equal(t, "apicrud", updated.Username)

	deleted, err := mr.DeleteAccount(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	gone, err := qr.Account(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

// TestAccountResolver_CreateAccount_GeneratesID when the resolver builds a model without ID.
func TestAccountResolver_CreateAccount_GeneratesID(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	created, err := mr.CreateAccount(ctx, "api-auto@example.com", "apiauto")
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.True(t, strings.HasPrefix(created.ID, "account_"))
}

// TestAccountResolver_Account_NotFound returns nil without error.
func TestAccountResolver_Account_NotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}

	none, err := qr.Account(ctx, "account_does_not_exist")
	require.NoError(t, err)
	assert.Nil(t, none)
}

// TestAccountResolver_DeleteAccount_NotFound returns false without error.
func TestAccountResolver_DeleteAccount_NotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	ok, err := mr.DeleteAccount(ctx, "account_never_created")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestAccountResolver_UpdateAccount_ValidationError propagates from the database layer.
func TestAccountResolver_UpdateAccount_ValidationError(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	_, err := mr.UpdateAccount(ctx, "any_id", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of email or username")
}

// TestResolver_createAccount_getAccount_updateAccount_deleteAccount covers the thin helpers on *Resolver.
func TestResolver_createAccount_getAccount_updateAccount_deleteAccount(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)

	acc, err := r.createAccount(ctx, "helper@example.com", "helperuser")
	require.NoError(t, err)
	require.NotNil(t, acc)

	got, err := r.getAccount(ctx, acc.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, acc.ID, got.ID)

	email := "helper2@example.com"
	upd, err := r.updateAccount(ctx, acc.ID, &email, nil)
	require.NoError(t, err)
	require.NotNil(t, upd)
	assert.Equal(t, email, upd.Email)

	ok, err := r.deleteAccount(ctx, acc.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	final, err := r.getAccount(ctx, acc.ID)
	require.NoError(t, err)
	assert.Nil(t, final)
}
