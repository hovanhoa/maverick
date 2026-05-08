package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPolicy(t *testing.T) {
	t.Parallel()

	t.Run("creates policy with single permission", func(t *testing.T) {
		perm := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		policy := NewPolicy(perm)

		assert.Len(t, policy.Permissions, 1)
		assert.Equal(t, perm, policy.Permissions[0])
	})

	t.Run("creates policy with multiple permissions", func(t *testing.T) {
		perm1 := NewPermission(scopeOrg, resourceWorkspace, actionRead)
		perm2 := NewPermission(scopeOrg, resourceAccount, actionWrite)
		perm3 := NewPermission(scopeOrg, resourceUser, actionDelete)

		policy := NewPolicy(perm1, perm2, perm3)

		assert.Len(t, policy.Permissions, 3)
		assert.Equal(t, perm1, policy.Permissions[0])
		assert.Equal(t, perm2, policy.Permissions[1])
		assert.Equal(t, perm3, policy.Permissions[2])
	})

	t.Run("creates empty policy with no permissions", func(t *testing.T) {
		policy := NewPolicy[testScope, testResource, testAction]()

		assert.Len(t, policy.Permissions, 0)
	})
}

func TestPolicyMatches(t *testing.T) {
	t.Parallel()

	t.Run("matches when permission is in policy", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionWrite),
		)

		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
	})

	t.Run("does not match when permission is not in policy", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
		)

		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
		assert.False(t, policy.Matches(NewPermission(scopeSelf, resourceAccount, actionRead)))
	})

	t.Run("matches with wildcard action in policy", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, testAction("*")),
		)

		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionWrite)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionDelete)))
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionRead)))
	})

	t.Run("matches with double wildcard in policy", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, testResource("*"), testAction("*")),
		)

		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceUser, actionDelete)))
	})

	t.Run("wildcard in policy does not match different scope", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, testResource("*"), testAction("*")),
		)

		assert.False(t, policy.Matches(NewPermission(scopeSelf, resourceWorkspace, actionRead)))
		assert.False(t, policy.Matches(NewPermission(scopeGlobal, resourceAccount, actionWrite)))
	})

	t.Run("empty policy matches nothing", func(t *testing.T) {
		policy := NewPolicy[testScope, testResource, testAction]()

		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
	})
}

func TestPolicyAdd(t *testing.T) {
	t.Parallel()

	t.Run("adds permission to empty policy", func(t *testing.T) {
		policy := NewPolicy[testScope, testResource, testAction]()
		assert.Len(t, policy.Permissions, 0)

		policy.Add(NewPermission(scopeOrg, resourceWorkspace, actionRead))
		assert.Len(t, policy.Permissions, 1)
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
	})

	t.Run("adds permission to existing policy", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionWrite),
		)
		assert.Len(t, policy.Permissions, 2)

		policy.Add(NewPermission(scopeOrg, resourceUser, actionDelete))
		assert.Len(t, policy.Permissions, 3)
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceUser, actionDelete)))
	})

	t.Run("adding duplicate permission does not deduplicate", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
		)
		assert.Len(t, policy.Permissions, 1)

		// Add the same permission again
		policy.Add(NewPermission(scopeOrg, resourceWorkspace, actionRead))
		// Should have 2 permissions (duplicates allowed)
		assert.Len(t, policy.Permissions, 2)
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
	})

	t.Run("adds wildcard permission", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
		)

		policy.Add(NewPermission(scopeOrg, resourceAccount, testAction("*")))
		assert.Len(t, policy.Permissions, 2)
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
	})

	t.Run("adds permissions with different scopes", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
		)

		policy.Add(NewPermission(scopeGlobal, resourceWorkspace, actionRead))
		policy.Add(NewPermission(scopeSelf, resourceWorkspace, actionRead))
		assert.Len(t, policy.Permissions, 3)
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeGlobal, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeSelf, resourceWorkspace, actionRead)))
	})
}

func TestPolicyRemove(t *testing.T) {
	t.Parallel()

	t.Run("removes exact matching permission", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionWrite),
			NewPermission(scopeOrg, resourceUser, actionDelete),
		)
		assert.Len(t, policy.Permissions, 3)

		policy.Remove(NewPermission(scopeOrg, resourceAccount, actionWrite))
		assert.Len(t, policy.Permissions, 2)
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceUser, actionDelete)))
	})

	t.Run("removes permission that does not exist has no effect", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionWrite),
		)
		assert.Len(t, policy.Permissions, 2)

		policy.Remove(NewPermission(scopeOrg, resourceUser, actionDelete))
		assert.Len(t, policy.Permissions, 2)
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
	})

	t.Run("removes all matching permissions when using wildcard", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceWorkspace, actionWrite),
			NewPermission(scopeOrg, resourceWorkspace, actionDelete),
			NewPermission(scopeOrg, resourceAccount, actionRead),
		)
		assert.Len(t, policy.Permissions, 4)

		// Remove all workspace permissions using wildcard
		policy.Remove(NewPermission(scopeOrg, resourceWorkspace, testAction("*")))
		assert.Len(t, policy.Permissions, 1)
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionWrite)))
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionDelete)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionRead)))
	})

	t.Run("removes wildcard permission", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, testAction("*")),
			NewPermission(scopeOrg, resourceAccount, actionRead),
		)
		assert.Len(t, policy.Permissions, 2)

		policy.Remove(NewPermission(scopeOrg, resourceWorkspace, testAction("*")))
		assert.Len(t, policy.Permissions, 1)
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionRead)))
	})

	t.Run("removes specific permission does not remove wildcard", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, testAction("*")),
			NewPermission(scopeOrg, resourceAccount, actionRead),
		)
		assert.Len(t, policy.Permissions, 2)

		// Try to remove a specific action, but wildcard should remain
		policy.Remove(NewPermission(scopeOrg, resourceWorkspace, actionRead))
		assert.Len(t, policy.Permissions, 2) // Wildcard still matches, but isn't removed
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
	})

	t.Run("removes from empty policy has no effect", func(t *testing.T) {
		policy := NewPolicy[testScope, testResource, testAction]()
		assert.Len(t, policy.Permissions, 0)

		policy.Remove(NewPermission(scopeOrg, resourceWorkspace, actionRead))
		assert.Len(t, policy.Permissions, 0)
	})

	t.Run("removes permission with different scope does not affect similar permissions", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeGlobal, resourceWorkspace, actionRead),
			NewPermission(scopeSelf, resourceWorkspace, actionRead),
		)
		assert.Len(t, policy.Permissions, 3)

		policy.Remove(NewPermission(scopeOrg, resourceWorkspace, actionRead))
		assert.Len(t, policy.Permissions, 2)
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeGlobal, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeSelf, resourceWorkspace, actionRead)))
	})

	t.Run("removes duplicate permissions", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceWorkspace, actionRead), // Duplicate
			NewPermission(scopeOrg, resourceAccount, actionWrite),
		)
		assert.Len(t, policy.Permissions, 3)

		// Remove should remove all matching permissions
		policy.Remove(NewPermission(scopeOrg, resourceWorkspace, actionRead))
		assert.Len(t, policy.Permissions, 1)
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
	})

	t.Run("removes using double wildcard", func(t *testing.T) {
		policy := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceWorkspace, actionWrite),
			NewPermission(scopeOrg, resourceAccount, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionWrite),
			NewPermission(scopeGlobal, resourceUser, actionDelete),
		)
		assert.Len(t, policy.Permissions, 5)

		// Remove all org permissions
		policy.Remove(NewPermission(scopeOrg, testResource("*"), testAction("*")))
		assert.Len(t, policy.Permissions, 1)
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.False(t, policy.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
		assert.True(t, policy.Matches(NewPermission(scopeGlobal, resourceUser, actionDelete)))
	})
}

func TestMerge(t *testing.T) {
	t.Parallel()

	t.Run("merges two policies with non-overlapping permissions", func(t *testing.T) {
		policy1 := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceWorkspace, actionWrite),
		)
		policy2 := NewPolicy(
			NewPermission(scopeOrg, resourceAccount, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionWrite),
		)

		merged := Merge(policy1, policy2)
		assert.Len(t, merged.Permissions, 4)
	})

	t.Run("merges policies with duplicate permissions", func(t *testing.T) {
		policy1 := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceWorkspace, actionWrite),
		)
		policy2 := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead), // Duplicate
			NewPermission(scopeOrg, resourceAccount, actionWrite),
		)

		merged := Merge(policy1, policy2)
		// Should deduplicate org:workspace:read
		assert.Len(t, merged.Permissions, 3)
	})

	t.Run("merges multiple policies", func(t *testing.T) {
		policy1 := NewPolicy(NewPermission(scopeOrg, resourceWorkspace, actionRead))
		policy2 := NewPolicy(NewPermission(scopeOrg, resourceAccount, actionRead))
		policy3 := NewPolicy(NewPermission(scopeOrg, resourceUser, actionRead))

		merged := Merge(policy1, policy2, policy3)
		assert.Len(t, merged.Permissions, 3)
	})

	t.Run("merged policy grants all permissions from source policies", func(t *testing.T) {
		policy1 := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceWorkspace, actionWrite),
		)
		policy2 := NewPolicy(
			NewPermission(scopeOrg, resourceAccount, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionWrite),
		)

		merged := Merge(policy1, policy2)

		// All permissions from both policies should be grantable
		assert.True(t, merged.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, merged.Matches(NewPermission(scopeOrg, resourceWorkspace, actionWrite)))
		assert.True(t, merged.Matches(NewPermission(scopeOrg, resourceAccount, actionRead)))
		assert.True(t, merged.Matches(NewPermission(scopeOrg, resourceAccount, actionWrite)))
	})

	t.Run("deduplication uses string representation", func(t *testing.T) {
		// Even if permissions are created separately, same scope:resource:action should be deduplicated
		policy1 := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
		)
		policy2 := NewPolicy(
			NewPermission(testScope("org"), testResource("workspace"), testAction("read")),
		)

		merged := Merge(policy1, policy2)
		assert.Len(t, merged.Permissions, 1) // Should be deduplicated
	})

	t.Run("merging with different scopes in the policies", func(t *testing.T) {
		policyOrgScope := NewPolicy(
			NewPermission(scopeOrg, resourceWorkspace, actionRead),
			NewPermission(scopeOrg, resourceAccount, actionRead),
		)
		policyGlobalScope := NewPolicy(
			NewPermission(scopeGlobal, resourceWorkspace, actionRead),
			NewPermission(scopeGlobal, resourceUser, actionWrite),
		)
		merged := Merge(policyOrgScope, policyGlobalScope)
		// Should contain all unique (scope, resource, action) combinations
		assert.Len(t, merged.Permissions, 4)
		assert.True(t, merged.Matches(NewPermission(scopeOrg, resourceWorkspace, actionRead)))
		assert.True(t, merged.Matches(NewPermission(scopeOrg, resourceAccount, actionRead)))
		assert.True(t, merged.Matches(NewPermission(scopeGlobal, resourceWorkspace, actionRead)))
		assert.True(t, merged.Matches(NewPermission(scopeGlobal, resourceUser, actionWrite)))
		// Should not match self scope
		assert.False(t, merged.Matches(NewPermission(scopeSelf, resourceWorkspace, actionRead)))
	})
}
