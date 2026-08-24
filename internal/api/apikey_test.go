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

	account, err := createTestAccount(mr, ctx, "self-key@example.com", "selfkey", nil, nil)
	require.NoError(t, err)

	selfCtx := asPrincipal(ctx, account.ID, model.RoleMember, "")

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

	target, err := createTestAccount(mr, ctx, "other-key-target@example.com", "otherkeytarget", nil, nil)
	require.NoError(t, err)

	caller, err := createTestAccount(mr, ctx, "other-key-caller@example.com", "otherkeycaller", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	_, err = mr.CreateAPIKey(memberCtx, target.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestAPIKeyResolver_CreateApiKey_ForOther_AllowedForAdmin(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	target, err := createTestAccount(mr, ctx, "admin-key-target@example.com", "adminkeytarget", nil, nil)
	require.NoError(t, err)

	caller, err := createTestAccount(mr, ctx, "admin-key-caller@example.com", "adminkeycaller", nil, nil)
	require.NoError(t, err)
	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, "")

	secret, err := mr.CreateAPIKey(adminCtx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, target.ID, secret.APIKey.AccountID)
}

// TestAPIKeyResolver_CreateApiKey_ForOther_DeniedForAdminOfAnotherTeam
// asserts the strict-per-team RBAC rule: an ADMIN of one team cannot issue
// an API key for an account belonging to a different team.
func TestAPIKeyResolver_CreateApiKey_ForOther_DeniedForAdminOfAnotherTeam(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	teamA, err := mr.CreateTeam(ctx, "apikey-team-a")
	require.NoError(t, err)
	teamB, err := mr.CreateTeam(ctx, "apikey-team-b")
	require.NoError(t, err)

	target, err := createTestAccount(mr, ctx, "target-key-in-b@example.com", "targetkeyinb", &teamB.ID, nil)
	require.NoError(t, err)

	caller, err := createTestAccount(mr, ctx, "admin-of-a-for-key@example.com", "adminofaforkey", &teamA.ID, nil)
	require.NoError(t, err)
	adminOfACtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, teamA.ID)

	_, err = mr.CreateAPIKey(adminOfACtx, target.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestAPIKeyResolver_ApiKeys_ListsMetadataOnly(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	account, err := createTestAccount(mr, ctx, "list-keys@example.com", "listkeys", nil, nil)
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

	account, err := createTestAccount(mr, ctx, "revoke-self@example.com", "revokeself", nil, nil)
	require.NoError(t, err)
	selfCtx := asPrincipal(ctx, account.ID, model.RoleMember, "")

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

	target, err := createTestAccount(mr, ctx, "revoke-target@example.com", "revoketarget", nil, nil)
	require.NoError(t, err)
	secret, err := mr.CreateAPIKey(ctx, target.ID)
	require.NoError(t, err)

	caller, err := createTestAccount(mr, ctx, "revoke-caller@example.com", "revokecaller", nil, nil)
	require.NoError(t, err)
	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

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

// TestAPIKeyResolver_UpdateApiKeyQuota_DeniedForSelf documents that, unlike
// createApiKey/revokeApiKey, setting a key's quota is never self-service -
// it's a governance control, so even the key's own account (as a plain
// MEMBER) can't adjust it.
func TestAPIKeyResolver_UpdateApiKeyQuota_DeniedForSelf(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	account, err := createTestAccount(mr, ctx, "quota-key-self@example.com", "quotakeyself", nil, nil)
	require.NoError(t, err)
	selfCtx := asPrincipal(ctx, account.ID, model.RoleMember, "")

	secret, err := mr.CreateAPIKey(selfCtx, account.ID)
	require.NoError(t, err)

	budget := 1000
	_, err = mr.UpdateAPIKeyQuota(selfCtx, secret.APIKey.ID, &budget, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestAPIKeyResolver_UpdateApiKeyQuota_AllowedForAdmin(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	team, err := mr.CreateTeam(ctx, "quota-key-team")
	require.NoError(t, err)

	target, err := createTestAccount(mr, ctx, "quota-key-target@example.com", "quotakeytarget", &team.ID, nil)
	require.NoError(t, err)

	admin, err := createTestAccount(mr, ctx, "quota-key-admin@example.com", "quotakeyadmin", &team.ID, nil)
	require.NoError(t, err)
	adminCtx := asPrincipal(ctx, admin.ID, model.RoleAdmin, team.ID)

	secret, err := mr.CreateAPIKey(adminCtx, target.ID)
	require.NoError(t, err)

	budget := 2500
	updated, err := mr.UpdateAPIKeyQuota(adminCtx, secret.APIKey.ID, &budget, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.MonthlyTokenBudget)
	assert.Equal(t, budget, *updated.MonthlyTokenBudget)
}

// TestAPIKeyResolver_UpdateApiKeyQuota_DeniedForAdminOfAnotherTeam mirrors
// the create/revoke RBAC rule: an ADMIN of one team cannot set the quota of
// a key belonging to an account on a different team.
func TestAPIKeyResolver_UpdateApiKeyQuota_DeniedForAdminOfAnotherTeam(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	teamA, err := mr.CreateTeam(ctx, "quota-key-team-a")
	require.NoError(t, err)
	teamB, err := mr.CreateTeam(ctx, "quota-key-team-b")
	require.NoError(t, err)

	target, err := createTestAccount(mr, ctx, "target-key-in-b-for-quota@example.com", "targetkeyinbforquota", &teamB.ID, nil)
	require.NoError(t, err)
	targetCtx := asPrincipal(ctx, target.ID, model.RoleMember, teamB.ID)
	secret, err := mr.CreateAPIKey(targetCtx, target.ID)
	require.NoError(t, err)

	caller, err := createTestAccount(mr, ctx, "admin-of-a-for-key-quota@example.com", "adminofaforkeyquota", &teamA.ID, nil)
	require.NoError(t, err)
	adminOfACtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, teamA.ID)

	budget := 1000
	_, err = mr.UpdateAPIKeyQuota(adminOfACtx, secret.APIKey.ID, &budget, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestAPIKeyResolver_UpdateApiKeyQuota_NotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	budget := 1000
	_, err := mr.UpdateAPIKeyQuota(ctx, "apikey_does_not_exist", &budget, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
