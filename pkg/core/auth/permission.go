package auth

import (
	"fmt"
)

// Permission represents a specific action that can be performed on a resource at a specific scope.
// It combines a Scope, Resource, and Action using Go generics, allowing callers to
// define their own scope, resource, and action types.
// Format: SCOPE:RESOURCE:ACTION
type Permission[Scope ~string, Resource ~string, Action ~string] struct {
	Scope    Scope
	Resource Resource
	Action   Action
}

// String returns the string representation of a permission in the format "scope:resource:action"
func (p Permission[Scope, Resource, Action]) String() string {
	return fmt.Sprintf("%s:%s:%s", p.Scope, p.Resource, p.Action)
}

// NewPermission creates a new permission from a scope, resource, and action.
// Scope is required and is the first parameter.
func NewPermission[Scope ~string, Resource ~string, Action ~string](scope Scope, resource Resource, action Action) Permission[Scope, Resource, Action] {
	return Permission[Scope, Resource, Action]{
		Scope:    scope,
		Resource: resource,
		Action:   action,
	}
}

// Matches checks if this permission matches another permission or pattern.
// Supports wildcards for resource and action only:
//   - "org:*:read" matches any resource with read action in org scope
//   - "org:workspace:*" matches any action on workspace in org scope
//   - "org:*:*" matches any resource and action in org scope
//
// Scope must match exactly - there is no wildcard support for scopes.
func (p Permission[Scope, Resource, Action]) Matches(pattern Permission[Scope, Resource, Action]) bool {
	scopeMatch := p.Scope == pattern.Scope
	resourceMatch := string(pattern.Resource) == "*" || p.Resource == pattern.Resource
	actionMatch := string(pattern.Action) == "*" || p.Action == pattern.Action

	return scopeMatch && resourceMatch && actionMatch
}
