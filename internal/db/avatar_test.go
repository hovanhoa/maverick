package db_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountAvatar_SetGetDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "avatar@example.com", Username: "avataruser"})
	require.NoError(t, err)

	none, err := database.GetAccountAvatar(ctx, account.ID)
	require.NoError(t, err)
	assert.Nil(t, none)

	data := []byte{0x89, 0x50, 0x4e, 0x47}
	require.NoError(t, database.SetAccountAvatar(ctx, account.ID, "image/png", data))

	found, err := database.GetAccountAvatar(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "image/png", found.ContentType)
	assert.Equal(t, data, found.Data)

	// Setting again overwrites rather than erroring or duplicating.
	newData := []byte{0xff, 0xd8, 0xff}
	require.NoError(t, database.SetAccountAvatar(ctx, account.ID, "image/jpeg", newData))
	updated, err := database.GetAccountAvatar(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "image/jpeg", updated.ContentType)
	assert.Equal(t, newData, updated.Data)

	deleted, err := database.DeleteAccountAvatar(ctx, account.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	goneAgain, err := database.DeleteAccountAvatar(ctx, account.ID)
	require.NoError(t, err)
	assert.False(t, goneAgain, "deleting an already-absent avatar is a no-op")

	afterDelete, err := database.GetAccountAvatar(ctx, account.ID)
	require.NoError(t, err)
	assert.Nil(t, afterDelete)
}
