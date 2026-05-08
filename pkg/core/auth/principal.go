package auth

import "slices"

// Principal represents an authenticated entity with their identity and permissions.
// It is the single source of truth for who is making a request and what they can do.
type Principal[Identity ~string, Role ~string] struct {
	// ID is the unique identifier for the principal (e.g., user ID)
	ID string

	// Type identifies the kind of principal (ADMIN, AGENCY_USER, AGENCY_CLIENT, etc.)
	// This is the principal's identity, not their role.
	Type Identity

	// Email is the principal's email address
	Email string

	// Name is the principal's full name
	Name string

	// OrgID is the organization/agency ID the principal belongs to
	OrgID string

	// OrgName is the organization/agency name
	OrgName string

	// Roles is the set of roles assigned to this principal.
	// Roles grant permissions and provide fine-grained access control.
	Roles []Role

	// Metadata stores additional extensible data about the principal
	Metadata map[string]interface{}
}

// HasIdentity checks if the principal has any of the specified identities.
// Returns true if the principal's type matches any of the provided identities.
func (p *Principal[Identity, Role]) HasIdentity(identities ...Identity) bool {
	if p == nil {
		return false
	}
	return slices.Contains(identities, p.Type)
}

// BelongsToOrg checks if the principal belongs to the specified organization.
func (p *Principal[Identity, Role]) BelongsToOrg(orgID string) bool {
	return p != nil && p.OrgID == orgID
}

// HasRole checks if the principal has any of the specified roles.
func (p *Principal[Identity, Role]) HasRole(roles ...Role) bool {
	if p == nil {
		return false
	}
	for _, role := range roles {
		if slices.Contains(p.Roles, role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the principal has all of the specified roles.
func (p *Principal[Identity, Role]) HasAllRoles(roles ...Role) bool {
	if p == nil {
		return false
	}
	for _, role := range roles {
		if !slices.Contains(p.Roles, role) {
			return false
		}
	}
	return true
}
