package api

import (
	"strings"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyResolver_CreateApiKey_Self_Allowed(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	account, err := mr.CreateAccount(ctx, "self-key@example.com", "selfkey", nil, nil)
	require.NoError(t, err)

	selfCtx := asPrincipal(ctx, account.ID, model.RoleMember)

	secret, err := mr.CreateAPIKey(selfCtx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, secret)
	assert.Equal(t, account.ID, secret.APIKey.AccountID)
	assert.NotEmpty(t, secret.Key)
	assert.True(t, strings.HasPrefix(secret.Key, secret.APIKey.Prefix))
}

func TestAPIKeyResolver_CreateApiKey_ForOther_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	target, err := mr.CreateAccount(ctx, "other-key-target@example.com", "otherkeytarget", nil, nil)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "other-key-caller@example.com", "otherkeycaller", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember)

	_, err = mr.CreateAPIKey(memberCtx, target.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestAPIKeyResolver_CreateApiKey_ForOther_AllowedForAdmin(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	target, err := mr.CreateAccount(ctx, "admin-key-target@example.com", "adminkeytarget", nil, nil)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "admin-key-caller@example.com", "adminkeycaller", nil, nil)
	require.NoError(t, err)
	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin)

	secret, err := mr.CreateAPIKey(adminCtx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, target.ID, secret.APIKey.AccountID)
}

func TestAPIKeyResolver_ApiKeys_ListsMetadataOnly(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	account, err := mr.CreateAccount(ctx, "list-keys@example.com", "listkeys", nil, nil)
	require.NoError(t, err)

	_, err = mr.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	keys, err := qr.APIKeys(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.NotEmpty(t, keys[0].Prefix)
	assert.Nil(t, keys[0].RevokedAt)
}

func TestAPIKeyResolver_RevokeApiKey_Self_Allowed(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	account, err := mr.CreateAccount(ctx, "revoke-self@example.com", "revokeself", nil, nil)
	require.NoError(t, err)
	selfCtx := asPrincipal(ctx, account.ID, model.RoleMember)

	secret, err := mr.CreateAPIKey(selfCtx, account.ID)
	require.NoError(t, err)

	revoked, err := mr.RevokeAPIKey(selfCtx, secret.APIKey.ID)
	require.NoError(t, err)
	assert.True(t, revoked)
}

func TestAPIKeyResolver_RevokeApiKey_ForOther_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	target, err := mr.CreateAccount(ctx, "revoke-target@example.com", "revoketarget", nil, nil)
	require.NoError(t, err)
	secret, err := mr.CreateAPIKey(ctx, target.ID)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "revoke-caller@example.com", "revokecaller", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember)

	_, err = mr.RevokeAPIKey(memberCtx, secret.APIKey.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestAPIKeyResolver_RevokeApiKey_NotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	ok, err := mr.RevokeAPIKey(ctx, "apikey_never_created")
	require.NoError(t, err)
	assert.False(t, ok)
}
