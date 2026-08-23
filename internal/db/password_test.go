package db_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetRandomAccountPassword_VerifyAccountPassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "pw@example.com", Username: "pwuser"})
	require.NoError(t, err)

	// No password set yet: verification must fail closed, not error.
	none, err := database.VerifyAccountPassword(ctx, "pwuser", "anything")
	require.NoError(t, err)
	assert.Nil(t, none)

	first, err := database.SetRandomAccountPassword(ctx, account.ID)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	found, err := database.VerifyAccountPassword(ctx, "pwuser", first)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, account.ID, found.ID)

	wrong, err := database.VerifyAccountPassword(ctx, "pwuser", "definitely-not-it")
	require.NoError(t, err)
	assert.Nil(t, wrong)

	unknown, err := database.VerifyAccountPassword(ctx, "no-such-user", first)
	require.NoError(t, err)
	assert.Nil(t, unknown)

	// Resetting invalidates the old password, not just adds a new one.
	second, err := database.SetRandomAccountPassword(ctx, account.ID)
	require.NoError(t, err)
	assert.NotEqual(t, first, second)

	staleFirst, err := database.VerifyAccountPassword(ctx, "pwuser", first)
	require.NoError(t, err)
	assert.Nil(t, staleFirst, "the old password must stop working after a reset")

	freshSecond, err := database.VerifyAccountPassword(ctx, "pwuser", second)
	require.NoError(t, err)
	require.NotNil(t, freshSecond)
}

func TestHashPassword_Deterministic(t *testing.T) {
	t.Parallel()

	hash, err := db.HashPassword("some-password")
	require.NoError(t, err)
	assert.NotEqual(t, "some-password", hash, "the hash must never equal the plaintext")
}
