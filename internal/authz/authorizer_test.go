package authz

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestAuthorizer_GetPrincipalFromEmail_NotApplicable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	authorizer := New(Dependencies{Database: database})

	principal, err := authorizer.GetPrincipalFromEmail(ctx, "anyone@example.com", "")
	require.NoError(t, err)
	assert.Nil(t, principal)
}
