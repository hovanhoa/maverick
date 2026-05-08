package auth

import (
	"slices"
)

// Policy represents a set of permissions that can be granted to a principal.
// Policies provide a way to group related permissions together.
type Policy[Scope ~string, Resource ~string, Action ~string] struct {
	Permissions []Permission[Scope, Resource, Action]
}

// NewPolicy creates a new policy with the given name and permissions
func NewPolicy[Scope ~string, Resource ~string, Action ~string](permissions ...Permission[Scope, Resource, Action]) Policy[Scope, Resource, Action] {
	return Policy[Scope, Resource, Action]{
		Permissions: permissions,
	}
}

// Matches checks if the policy grants access for a specific permission.
// This supports wildcard matching.
func (p Policy[Scope, Resource, Action]) Matches(permission Permission[Scope, Resource, Action]) bool {
	return slices.ContainsFunc(p.Permissions, permission.Matches)
}

// Remove removes a permission from the policy.
func (p *Policy[Scope, Resource, Action]) Remove(permission Permission[Scope, Resource, Action]) {
	p.Permissions = slices.DeleteFunc(p.Permissions, func(p Permission[Scope, Resource, Action]) bool {
		return p.Matches(permission)
	})
}

// Add adds a permission to the policy.
func (p *Policy[Scope, Resource, Action]) Add(permission Permission[Scope, Resource, Action]) {
	p.Permissions = append(p.Permissions, permission)
}

// Merge combines multiple policies into one
func Merge[Scope ~string, Resource ~string, Action ~string](policies ...Policy[Scope, Resource, Action]) Policy[Scope, Resource, Action] {
	permMap := make(map[string]Permission[Scope, Resource, Action])

	for _, policy := range policies {
		for _, perm := range policy.Permissions {
			permMap[perm.String()] = perm
		}
	}

	permissions := make([]Permission[Scope, Resource, Action], 0, len(permMap))
	for _, perm := range permMap {
		permissions = append(permissions, perm)
	}

	return Policy[Scope, Resource, Action]{
		Permissions: permissions,
	}
}
