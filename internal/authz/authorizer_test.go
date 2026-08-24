package authz

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/driver/memkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestTokenService returns a TokenService backed by an in-memory KV
// store and a freshly generated signing key.
func newTestTokenService(t *testing.T) *auth.TokenService[model.Identity] {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	return auth.NewTokenService[model.Identity](auth.Dependencies{
		DB:            memkv.New(),
		JWTPrivateKey: privateKey,
		JWTPublicKey:  &privateKey.PublicKey,
	})
}

func TestAuthorizer_GetPrincipalFromToken_ValidKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	authorizer := New(Dependencies{Database: database})

	account, err := database.CreateAccount(ctx, &model.Account{
		Email:    "authz@example.com",
		Username: "authzuser",
		Role:     model.RoleAdmin,
	})
	require.NoError(t, err)

	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	principal, err := authorizer.GetPrincipalFromToken(ctx, auth.TokenString(secret.Key), "")
	require.NoError(t, err)
	require.NotNil(t, principal)
	assert.Equal(t, account.ID, principal.ID)
	assert.Equal(t, model.IdentityAccount, principal.Type)
	assert.Equal(t, account.Email, principal.Email)
	assert.True(t, principal.HasRole(model.RoleAdmin))
	assert.False(t, principal.HasRole(model.RoleOwner))

	keyID, ok := model.APIKeyIDFromPrincipal(principal)
	require.True(t, ok, "a principal resolved from an API key must carry that key's id, for per-key quota enforcement")
	assert.Equal(t, secret.APIKey.ID, keyID)
}

// TestAuthorizer_GetPrincipalFromToken_SessionJWTHasNoAPIKeyID documents
// that a session JWT - unlike an API key - isn't tied to any one credential
// row, so APIKeyIDFromPrincipal must report false for it (per-key quota
// only applies to calls actually authenticated by an API key).
func TestAuthorizer_GetPrincipalFromToken_SessionJWTHasNoAPIKeyID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	tokens := newTestTokenService(t)
	authorizer := New(Dependencies{Database: database, Tokens: tokens})

	account, err := database.CreateAccount(ctx, &model.Account{Email: "nokeyid@example.com", Username: "nokeyiduser"})
	require.NoError(t, err)

	jwt, err := tokens.GenerateJWT(ctx, account.ID, account.Email, account.Username, "", "", model.IdentityAccount)
	require.NoError(t, err)

	principal, err := authorizer.GetPrincipalFromToken(ctx, jwt.Token, "")
	require.NoError(t, err)
	require.NotNil(t, principal)

	_, ok := model.APIKeyIDFromPrincipal(principal)
	assert.False(t, ok)
}

func TestAuthorizer_GetPrincipalFromToken_InvalidKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	authorizer := New(Dependencies{Database: database})

	principal, err := authorizer.GetPrincipalFromToken(ctx, auth.TokenString("not-a-real-key"), "")
	require.Error(t, err)
	assert.Nil(t, principal)
}

func TestAuthorizer_GetPrincipalFromToken_RevokedKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	authorizer := New(Dependencies{Database: database})

	account, err := database.CreateAccount(ctx, &model.Account{Email: "revoked@example.com", Username: "revokeduser"})
	require.NoError(t, err)

	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	revoked, err := database.RevokeAPIKey(ctx, secret.APIKey.ID)
	require.NoError(t, err)
	require.True(t, revoked)

	principal, err := authorizer.GetPrincipalFromToken(ctx, auth.TokenString(secret.Key), "")
	require.Error(t, err)
	assert.Nil(t, principal)
}

func TestAuthorizer_GetPrincipalFromToken_ValidSessionJWT(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	tokens := newTestTokenService(t)
	authorizer := New(Dependencies{Database: database, Tokens: tokens})

	account, err := database.CreateAccount(ctx, &model.Account{
		Email:    "session@example.com",
		Username: "sessionuser",
		Role:     model.RoleAdmin,
	})
	require.NoError(t, err)

	jwt, err := tokens.GenerateJWT(ctx, account.ID, account.Email, account.Username, "", "", model.IdentityAccount)
	require.NoError(t, err)

	principal, err := authorizer.GetPrincipalFromToken(ctx, jwt.Token, "")
	require.NoError(t, err)
	require.NotNil(t, principal)
	assert.Equal(t, account.ID, principal.ID)
	assert.Equal(t, model.IdentityAccount, principal.Type)
	assert.Equal(t, account.Email, principal.Email)
	assert.True(t, principal.HasRole(model.RoleAdmin))
}

// TestAuthorizer_GetPrincipalFromToken_SessionJWTReflectsRoleChanges verifies
// that a role change made after login takes effect on the very next request,
// since the account is re-fetched on every session-JWT verification rather
// than trusting whatever role the JWT was minted with.
func TestAuthorizer_GetPrincipalFromToken_SessionJWTReflectsRoleChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	tokens := newTestTokenService(t)
	authorizer := New(Dependencies{Database: database, Tokens: tokens})

	account, err := database.CreateAccount(ctx, &model.Account{
		Email:    "promoted@example.com",
		Username: "promoteduser",
		Role:     model.RoleMember,
	})
	require.NoError(t, err)

	jwt, err := tokens.GenerateJWT(ctx, account.ID, account.Email, account.Username, "", "", model.IdentityAccount)
	require.NoError(t, err)

	ownerRole := model.RoleOwner
	_, err = database.UpdateAccount(ctx, account.ID, nil, nil, nil, nil, nil, &ownerRole)
	require.NoError(t, err)

	principal, err := authorizer.GetPrincipalFromToken(ctx, jwt.Token, "")
	require.NoError(t, err)
	require.NotNil(t, principal)
	assert.True(t, principal.HasRole(model.RoleOwner))
	assert.False(t, principal.HasRole(model.RoleMember))
}

func TestAuthorizer_GetPrincipalFromToken_RevokedSessionJWT(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	tokens := newTestTokenService(t)
	authorizer := New(Dependencies{Database: database, Tokens: tokens})

	account, err := database.CreateAccount(ctx, &model.Account{Email: "revokedsession@example.com", Username: "revokedsessionuser"})
	require.NoError(t, err)

	jwt, err := tokens.GenerateJWT(ctx, account.ID, account.Email, account.Username, "", "", model.IdentityAccount)
	require.NoError(t, err)
	require.NoError(t, tokens.RevokeJWT(ctx, jwt.ID))

	principal, err := authorizer.GetPrincipalFromToken(ctx, jwt.Token, "")
	require.Error(t, err)
	assert.Nil(t, principal)
}

// TestAuthorizer_GetPrincipalFromToken_NoTokenServiceFallsBackToAPIKey
// documents that a nil Tokens dependency (e.g. an Authorizer built without
// session support) doesn't panic on a non-API-key-shaped token - it's
// treated as an invalid API key instead.
func TestAuthorizer_GetPrincipalFromToken_NoTokenServiceFallsBackToAPIKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	authorizer := New(Dependencies{Database: database})

	principal, err := authorizer.GetPrincipalFromToken(ctx, auth.TokenString("not-a-jwt-either"), "")
	require.Error(t, err)
	assert.Nil(t, principal)
}

func TestAuthorizer_GetPrincipalFromEmail_NotApplicable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	authorizer := New(Dependencies{Database: database})

	principal, err := authorizer.GetPrincipalFromEmail(ctx, "anyone@example.com", "")
	require.NoError(t, err)
	assert.Nil(t, principal)
}
