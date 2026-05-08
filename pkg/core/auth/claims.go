package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the claims of a JWT token.
type Claims[Identity ~string] struct {
	jwt.RegisteredClaims

	// SubjectType is the type of the subject, used to resolve the type of entity
	// that the JWT is authenticating. Readers should combine this with the
	// subject to resolve the subject details.
	SubjectType      Identity `json:"typ"`
	SubjectFullName  string   `json:"nam"`
	SubjectEmail     string   `json:"adr"`
	OrganizationID   string   `json:"org_id"`
	OrganizationName string   `json:"org_name"`
}

// ClaimsToPrincipal converts JWT claims into a Principal.
// This is the bridge between token-based authentication and the principal model.
// It allows the new Principal-based system to work with existing JWT claims.
func ClaimsToPrincipal[Identity ~string, Role ~string](claims *Claims[Identity]) *Principal[Identity, Role] {
	if claims == nil {
		return nil
	}

	return &Principal[Identity, Role]{
		ID:       claims.Subject,
		Type:     claims.SubjectType,
		Email:    claims.SubjectEmail,
		Name:     claims.SubjectFullName,
		OrgID:    claims.OrganizationID,
		OrgName:  claims.OrganizationName,
		Metadata: make(map[string]interface{}),
	}
}

// WithRoles sets the roles of the principal.
func (p Principal[Identity, Role]) WithRoles(roles ...Role) *Principal[Identity, Role] {
	p.Roles = roles
	return &p
}

// WithIdentity sets the identity of the principal.
func (p Principal[Identity, Role]) WithIdentity(identity Identity) *Principal[Identity, Role] {
	p.Type = identity
	return &p
}
