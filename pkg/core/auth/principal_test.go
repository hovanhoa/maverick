package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test identity and role types
type testIdentity string
type testRole string

const (
	identityAdmin        testIdentity = "ADMIN"
	identityAgencyUser   testIdentity = "AGENCY_USER"
	identityAgencyClient testIdentity = "AGENCY_CLIENT"
	identityUser         testIdentity = "USER"
	identityAnonymous    testIdentity = "ANONYMOUS"
)

const (
	roleUser  testRole = "USER"
	roleAdmin testRole = "ADMIN"
)

func TestPrincipalHasIdentity(t *testing.T) {
	t.Parallel()

	t.Run("returns true when principal has the identity", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:   "user_123",
			Type: identityAgencyUser,
		}

		assert.True(t, principal.HasIdentity(identityAgencyUser))
	})

	t.Run("returns false when principal does not have the identity", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:   "user_123",
			Type: identityAgencyUser,
		}

		assert.False(t, principal.HasIdentity(identityAdmin))
	})

	t.Run("returns true when principal has one of multiple identities", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:   "user_123",
			Type: identityAgencyClient,
		}

		assert.True(t, principal.HasIdentity(identityAdmin, identityAgencyClient, identityUser))
	})

	t.Run("returns false when principal has none of the identities", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:   "user_123",
			Type: identityAgencyUser,
		}

		assert.False(t, principal.HasIdentity(identityAdmin, identityAgencyClient))
	})

	t.Run("returns false for nil principal", func(t *testing.T) {
		var principal *Principal[testIdentity, testRole]

		assert.False(t, principal.HasIdentity(identityAgencyUser))
	})

}

func TestPrincipalBelongsToOrg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		principalID string
		checkOrgID  string
		wantMatch   bool
	}{
		{
			name:        "matching organization",
			principalID: "org_abc",
			checkOrgID:  "org_abc",
			wantMatch:   true,
		},
		{
			name:        "different organization",
			principalID: "org_abc",
			checkOrgID:  "org_xyz",
			wantMatch:   false,
		},
		{
			name:        "empty org ID check against non-empty",
			principalID: "org_abc",
			checkOrgID:  "",
			wantMatch:   false,
		},
		{
			name:        "both org IDs empty",
			principalID: "",
			checkOrgID:  "",
			wantMatch:   true,
		},
		{
			name:        "case-sensitive uppercase",
			principalID: "org_abc",
			checkOrgID:  "ORG_ABC",
			wantMatch:   false,
		},
		{
			name:        "case-sensitive mixed case",
			principalID: "org_abc",
			checkOrgID:  "Org_abc",
			wantMatch:   false,
		},
		{
			name:        "partial match prefix",
			principalID: "org_abc",
			checkOrgID:  "org_ab",
			wantMatch:   false,
		},
		{
			name:        "partial match suffix",
			principalID: "org_abc",
			checkOrgID:  "org_abcd",
			wantMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal := &Principal[testIdentity, testRole]{
				ID:    "user_123",
				OrgID: tt.principalID,
			}
			assert.Equal(t, tt.wantMatch, principal.BelongsToOrg(tt.checkOrgID))
		})
	}

	t.Run("returns false for nil principal", func(t *testing.T) {
		var principal *Principal[testIdentity, testRole]
		assert.False(t, principal.BelongsToOrg("org_abc"))
	})
}

func TestPrincipalHasRole(t *testing.T) {
	t.Parallel()

	t.Run("returns true when principal has the role", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser, roleAdmin},
		}

		assert.True(t, principal.HasRole(roleUser))
	})

	t.Run("returns false when principal does not have the role", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser},
		}

		assert.False(t, principal.HasRole(roleAdmin))
	})

	t.Run("returns true when principal has one of multiple roles", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser},
		}

		assert.True(t, principal.HasRole(roleAdmin, roleUser))
	})

	t.Run("returns false when principal has none of the roles", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser},
		}

		// Checking for roles that don't exist
		assert.False(t, principal.HasRole(testRole("EDITOR"), testRole("VIEWER")))
	})

	t.Run("returns false for nil principal", func(t *testing.T) {
		var principal *Principal[testIdentity, testRole]

		assert.False(t, principal.HasRole(roleUser))
	})

	t.Run("returns false when principal has no roles", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{},
		}

		assert.False(t, principal.HasRole(roleUser))
	})

}

func TestPrincipalHasAllRoles(t *testing.T) {
	t.Parallel()

	t.Run("returns true when principal has all specified roles", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser, roleAdmin},
		}

		assert.True(t, principal.HasAllRoles(roleUser, roleAdmin))
	})

	t.Run("returns false when principal is missing one role", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser},
		}

		assert.False(t, principal.HasAllRoles(roleUser, roleAdmin))
	})

	t.Run("returns false when principal has none of the roles", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser},
		}

		assert.False(t, principal.HasAllRoles(testRole("EDITOR"), testRole("VIEWER")))
	})

	t.Run("returns true when checking single role", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{roleUser, roleAdmin},
		}

		assert.True(t, principal.HasAllRoles(roleUser))
	})

	t.Run("returns false for nil principal", func(t *testing.T) {
		var principal *Principal[testIdentity, testRole]

		assert.False(t, principal.HasAllRoles(roleUser))
	})

	t.Run("returns false when principal has no roles but roles are required", func(t *testing.T) {
		principal := &Principal[testIdentity, testRole]{
			ID:    "user_123",
			Roles: []testRole{},
		}

		assert.False(t, principal.HasAllRoles(roleUser))
	})

}
