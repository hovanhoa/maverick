package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test scope, resource and action types
type testScope string
type testResource string
type testAction string

const (
	resourceWorkspace testResource = "workspace"
	resourceAccount   testResource = "account"
	resourceUser      testResource = "user"
)

const (
	actionRead   testAction = "read"
	actionWrite  testAction = "write"
	actionDelete testAction = "delete"
)

const (
	scopeOrg    testScope = "org"
	scopeGlobal testScope = "global"
	scopeSelf   testScope = "self"
)

func TestNewPermission(t *testing.T) {
	t.Parallel()

	perm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
	assert.Equal(t, scopeOrg, perm.Scope)
	assert.Equal(t, resourceWorkspace, perm.Resource)
	assert.Equal(t, actionRead, perm.Action)
}

func TestPermissionString(t *testing.T) {
	t.Parallel()

	t.Run("formats permission as scope:resource:action", func(t *testing.T) {
		perm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		assert.Equal(t, "org:workspace:read", perm.String())
	})

	t.Run("formats permission with special characters", func(t *testing.T) {
		perm := NewPermission(testScope("my-scope"), testResource("my-resource"), testAction("my_action"))
		assert.Equal(t, "my-scope:my-resource:my_action", perm.String())
	})

	t.Run("formats different scopes correctly", func(t *testing.T) {
		permOrg := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		permGlobal := NewPermission(scopeGlobal, resourceWorkspace, actionRead)
		permSelf := NewPermission(scopeSelf, resourceWorkspace, actionRead)

		assert.Equal(t, "org:workspace:read", permOrg.String())
		assert.Equal(t, "global:workspace:read", permGlobal.String())
		assert.Equal(t, "self:workspace:read", permSelf.String())
	})
}

func TestPermissionMatches(t *testing.T) {
	t.Parallel()

	t.Run("exact match", func(t *testing.T) {
		perm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		pattern := NewPermission(scopeOrg, resourceWorkspace, actionRead)

		assert.True(t, perm.Matches(pattern))
	})

	t.Run("different scope does not match", func(t *testing.T) {
		perm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		pattern := NewPermission(scopeSelf, resourceWorkspace, actionRead)

		assert.False(t, perm.Matches(pattern))
	})

	t.Run("different resource does not match", func(t *testing.T) {
		perm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		pattern := NewPermission(scopeOrg, resourceAccount, actionRead)

		assert.False(t, perm.Matches(pattern))
	})

	t.Run("different action does not match", func(t *testing.T) {
		perm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		pattern := NewPermission(scopeOrg, resourceWorkspace, actionWrite)

		assert.False(t, perm.Matches(pattern))
	})

	t.Run("wildcard resource matches any resource", func(t *testing.T) {
		workspacePerm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		accountPerm := NewPermission(scopeOrg, resourceAccount, actionRead)
		pattern := NewPermission(scopeOrg, testResource("*"), actionRead)

		assert.True(t, workspacePerm.Matches(pattern))
		assert.True(t, accountPerm.Matches(pattern))
	})

	t.Run("wildcard action matches any action", func(t *testing.T) {
		readPerm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		writePerm := NewPermission(scopeOrg, resourceWorkspace, actionWrite)
		pattern := NewPermission(scopeOrg, resourceWorkspace, testAction("*"))

		assert.True(t, readPerm.Matches(pattern))
		assert.True(t, writePerm.Matches(pattern))
	})

	t.Run("double wildcard matches everything in same scope", func(t *testing.T) {
		workspaceRead := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		accountWrite := NewPermission(scopeOrg, resourceAccount, actionWrite)
		userDelete := NewPermission(scopeOrg, resourceUser, actionDelete)
		pattern := NewPermission(scopeOrg, testResource("*"), testAction("*"))

		assert.True(t, workspaceRead.Matches(pattern))
		assert.True(t, accountWrite.Matches(pattern))
		assert.True(t, userDelete.Matches(pattern))
	})

	t.Run("wildcard does not match across different scopes", func(t *testing.T) {
		permOrg := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		patternSelf := NewPermission(scopeSelf, testResource("*"), testAction("*"))

		assert.False(t, permOrg.Matches(patternSelf))
	})

}

func TestPermissionScope(t *testing.T) {
	t.Parallel()

	t.Run("permission has the assigned scope", func(t *testing.T) {
		permOrg := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		permGlobal := NewPermission(scopeGlobal, resourceWorkspace, actionRead)
		permSelf := NewPermission(scopeSelf, resourceWorkspace, actionRead)

		assert.Equal(t, scopeOrg, permOrg.Scope)
		assert.Equal(t, scopeGlobal, permGlobal.Scope)
		assert.Equal(t, scopeSelf, permSelf.Scope)
	})

	t.Run("different permissions with different scopes are distinct", func(t *testing.T) {
		permOrg := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		permSelf := NewPermission(scopeSelf, resourceWorkspace, actionRead)

		assert.NotEqual(t, permOrg.Scope, permSelf.Scope)
		assert.NotEqual(t, permOrg.String(), permSelf.String())
	})
}
