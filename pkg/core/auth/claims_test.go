package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimsToPrincipal(t *testing.T) {
	principal := ClaimsToPrincipal[testIdentity, testRole](&Claims[testIdentity]{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user_123",
			ID:      "jwt_abc",
		},
		SubjectType:      identityAgencyUser,
		SubjectFullName:  "John Doe",
		SubjectEmail:     "john@example.com",
		OrganizationID:   "org_abc",
		OrganizationName: "Acme Corp",
	})

	require.NotNil(t, principal)
	assert.Equal(t, "user_123", principal.ID)
	assert.Equal(t, identityAgencyUser, principal.Type)
	assert.Equal(t, "john@example.com", principal.Email)
	assert.Equal(t, "John Doe", principal.Name)
	assert.Equal(t, "org_abc", principal.OrgID)
	assert.Equal(t, "Acme Corp", principal.OrgName)
	assert.NotNil(t, principal.Metadata)
	assert.Len(t, principal.Metadata, 0)
	assert.Nil(t, principal.Roles)

	principal = ClaimsToPrincipal[testIdentity, testRole](nil)
	assert.Nil(t, principal)
}
