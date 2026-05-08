package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithPrincipal(t *testing.T) {
	t.Parallel()

	t.Run("adds principal to context", func(t *testing.T) {
		ctx := context.Background()
		principal := &Principal[testIdentity, testRole]{
			ID:      "user_123",
			Type:    identityAgencyUser,
			Email:   "test@example.com",
			Name:    "Test User",
			OrgID:   "org_abc",
			OrgName: "Test Org",
		}

		newCtx := WithPrincipal(ctx, principal)

		assert.NotNil(t, newCtx)
		assert.NotEqual(t, ctx, newCtx)
	})

	t.Run("overwrites previous principal", func(t *testing.T) {
		ctx := context.Background()

		principal1 := &Principal[testIdentity, testRole]{
			ID:   "user_1",
			Type: identityUser,
		}

		principal2 := &Principal[testIdentity, testRole]{
			ID:   "user_2",
			Type: identityAdmin,
		}

		ctx1 := WithPrincipal(ctx, principal1)
		ctx2 := WithPrincipal(ctx1, principal2)

		retrieved := GetPrincipal[testIdentity, testRole](ctx2)

		require.NotNil(t, retrieved)
		assert.Equal(t, "user_2", retrieved.ID)
		assert.Equal(t, identityAdmin, retrieved.Type)
	})

	t.Run("stores principal with all fields", func(t *testing.T) {
		ctx := context.Background()
		principal := &Principal[testIdentity, testRole]{
			ID:      "user_123",
			Type:    identityAgencyUser,
			Email:   "test@example.com",
			Name:    "Test User",
			OrgID:   "org_abc",
			OrgName: "Test Org",
			Roles:   []testRole{roleUser, roleAdmin},
			Metadata: map[string]interface{}{
				"last_login": "2024-01-15",
			},
		}

		newCtx := WithPrincipal(ctx, principal)
		retrieved := GetPrincipal[testIdentity, testRole](newCtx)

		require.NotNil(t, retrieved)
		assert.Equal(t, principal.ID, retrieved.ID)
		assert.Equal(t, principal.Type, retrieved.Type)
		assert.Equal(t, principal.Email, retrieved.Email)
		assert.Equal(t, principal.Name, retrieved.Name)
		assert.Equal(t, principal.OrgID, retrieved.OrgID)
		assert.Equal(t, principal.OrgName, retrieved.OrgName)
		assert.Equal(t, principal.Roles, retrieved.Roles)
		assert.Equal(t, principal.Metadata, retrieved.Metadata)
	})

}

func TestGetPrincipal(t *testing.T) {
	t.Parallel()

	t.Run("retrieves principal from context", func(t *testing.T) {
		ctx := context.Background()
		principal := &Principal[testIdentity, testRole]{
			ID:      "user_123",
			Type:    identityAgencyUser,
			Email:   "test@example.com",
			Name:    "Test User",
			OrgID:   "org_abc",
			OrgName: "Test Org",
		}

		ctx = WithPrincipal(ctx, principal)
		retrieved := GetPrincipal[testIdentity, testRole](ctx)

		require.NotNil(t, retrieved)
		assert.Equal(t, "user_123", retrieved.ID)
		assert.Equal(t, identityAgencyUser, retrieved.Type)
	})

	t.Run("returns nil when no principal in context", func(t *testing.T) {
		ctx := context.Background()

		retrieved := GetPrincipal[testIdentity, testRole](ctx)

		assert.Nil(t, retrieved)
	})

	t.Run("returns nil for context without principal key", func(t *testing.T) {
		type testKey string
		const (
			key testKey = "key"
		)
		ctx := context.WithValue(context.Background(), key, "some_value")

		retrieved := GetPrincipal[testIdentity, testRole](ctx)

		assert.Nil(t, retrieved)
	})

	t.Run("returns correct principal from nested contexts", func(t *testing.T) {
		baseCtx := context.Background()

		type testKey string
		const (
			key1 testKey = "key1"
			key2 testKey = "key2"
		)

		// Add some other values to context
		ctx1 := context.WithValue(baseCtx, key1, "value1")
		ctx2 := context.WithValue(ctx1, key2, "value2")

		// Add principal
		principal := &Principal[testIdentity, testRole]{
			ID:   "user_123",
			Type: identityUser,
		}
		ctx3 := WithPrincipal(ctx2, principal)

		retrieved := GetPrincipal[testIdentity, testRole](ctx3)

		require.NotNil(t, retrieved)
		assert.Equal(t, "user_123", retrieved.ID)
	})

}
