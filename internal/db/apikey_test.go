package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAPIKey_HashLookupRevoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "key@example.com", Username: "keyuser"})
	require.NoError(t, err)

	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, secret)
	assert.True(t, strings.HasPrefix(secret.APIKey.ID, "apikey_"))
	assert.Equal(t, account.ID, secret.APIKey.AccountID)
	assert.NotEmpty(t, secret.Key)
	assert.True(t, strings.HasPrefix(secret.Key, secret.APIKey.Prefix))
	assert.Nil(t, secret.APIKey.RevokedAt)
	assert.Nil(t, secret.APIKey.LastUsedAt, "a freshly issued key has never been used")

	// A valid, non-revoked key resolves back to its account (and itself) by
	// hash, and that lookup records the key as just having been used.
	found, foundKey, err := database.GetAccountByAPIKeyHash(ctx, db.HashAPIKey(secret.Key))
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, account.ID, found.ID)
	require.NotNil(t, foundKey)
	assert.Equal(t, secret.APIKey.ID, foundKey.ID)

	touched, err := database.GetAPIKeyByID(ctx, secret.APIKey.ID)
	require.NoError(t, err)
	require.NotNil(t, touched.LastUsedAt, "lookup by hash must stamp last_used_at")

	// An invalid key (never issued) resolves to nothing.
	none, noneKey, err := database.GetAccountByAPIKeyHash(ctx, db.HashAPIKey("not-a-real-key"))
	require.NoError(t, err)
	assert.Nil(t, none)
	assert.Nil(t, noneKey)

	// Revoking succeeds exactly once.
	revoked, err := database.RevokeAPIKey(ctx, secret.APIKey.ID)
	require.NoError(t, err)
	assert.True(t, revoked)

	revokedAgain, err := database.RevokeAPIKey(ctx, secret.APIKey.ID)
	require.NoError(t, err)
	assert.False(t, revokedAgain, "revoking an already-revoked key is a no-op")

	// A revoked key no longer resolves to its account.
	goneAfterRevoke, _, err := database.GetAccountByAPIKeyHash(ctx, db.HashAPIKey(secret.Key))
	require.NoError(t, err)
	assert.Nil(t, goneAfterRevoke, "revoked key must fail auth")
}

func TestCreateAPIKey_AccountNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	_, err := database.CreateAPIKey(ctx, "account_does_not_exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account not found")
}

func TestListAPIKeysByAccount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "list@example.com", Username: "listuser"})
	require.NoError(t, err)

	empty, err := database.ListAPIKeysByAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, empty)

	first, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)
	second, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	keys, err := database.ListAPIKeysByAccount(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, []string{second.APIKey.ID, first.APIKey.ID}, []string{keys[0].ID, keys[1].ID}, "most recently created first")

	// Metadata only: the plaintext secret is never persisted or returned.
	for _, key := range keys {
		assert.NotContains(t, key.Prefix, "not-a-real-secret")
	}
}

func TestGetAPIKeyByID_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	none, err := database.GetAPIKeyByID(ctx, "apikey_does_not_exist")
	require.NoError(t, err)
	assert.Nil(t, none)
}

func TestRevokeAPIKey_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	ok, err := database.RevokeAPIKey(ctx, "apikey_never_created")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestUpdateAPIKeyQuota(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "keyquota@example.com", Username: "keyquotauser"})
	require.NoError(t, err)
	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)
	assert.Nil(t, secret.APIKey.MonthlyTokenBudget, "new keys start unlimited")

	budget := 500_000
	updated, err := database.UpdateAPIKeyQuota(ctx, secret.APIKey.ID, &budget, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.MonthlyTokenBudget)
	assert.Equal(t, budget, *updated.MonthlyTokenBudget)

	fetched, err := database.GetAPIKeyByID(ctx, secret.APIKey.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.MonthlyTokenBudget)
	assert.Equal(t, budget, *fetched.MonthlyTokenBudget)

	clear := true
	cleared, err := database.UpdateAPIKeyQuota(ctx, secret.APIKey.ID, nil, &clear)
	require.NoError(t, err)
	assert.Nil(t, cleared.MonthlyTokenBudget)
}

func TestUpdateAPIKeyQuota_RequiresOneField(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "keyquota2@example.com", Username: "keyquotauser2"})
	require.NoError(t, err)
	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	_, err = database.UpdateAPIKeyQuota(ctx, secret.APIKey.ID, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of")
}

func TestUpdateAPIKeyQuota_RejectsSettingAndClearingTogether(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "keyquota3@example.com", Username: "keyquotauser3"})
	require.NoError(t, err)
	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	budget := 100
	clear := true
	_, err = database.UpdateAPIKeyQuota(ctx, secret.APIKey.ID, &budget, &clear)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot set monthlyTokenBudget and clearMonthlyTokenBudget")
}

func TestUpdateAPIKeyQuota_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	budget := 100
	_, err := database.UpdateAPIKeyQuota(ctx, "apikey_missing", &budget, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api key not found")
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	t.Parallel()

	assert.Equal(t, db.HashAPIKey("secret"), db.HashAPIKey("secret"))
	assert.NotEqual(t, db.HashAPIKey("secret-a"), db.HashAPIKey("secret-b"))
}
