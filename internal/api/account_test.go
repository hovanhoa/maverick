package api

import (
	"context"
	"strings"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testResolver returns a Resolver backed by a fresh test database, with the
// context authenticated as an OWNER so existing CRUD-focused tests don't
// need to think about RBAC. The principal is not backed by a real account
// row (so it never shows up in account list/count assertions); createTeam's
// auto-OWNER assignment skips callers with no matching account, just as it
// would need to for any RBAC-exempt caller. Tests exercising RBAC itself,
// or the auto-OWNER assignment itself, should install their own principal
// via auth.WithPrincipal against a real, persisted account.
//
// This default principal has no team (OrgID ""), so it's only "exempt"
// from plain role checks (requireRole) - team-scoped operations
// (requireTeamMember/requireTeamRole) still deny it, since OWNER/ADMIN are
// scoped to the team you belong to, not global. A test that needs to act
// on a specific team it just created should scope a fresh context to that
// team via asPrincipal(ctx, "account_test_caller", model.RoleOwner,
// teamID) rather than reusing this one.
func testResolver(t *testing.T) (*Resolver, context.Context) {
	t.Helper()
	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	principal := &auth.Principal[model.Identity, model.Role]{
		ID:   "account_test_caller",
		Type: model.IdentityAccount,
	}
	ctx = auth.WithPrincipal(ctx, principal.WithRoles(model.RoleOwner))

	return &Resolver{deps: Dependencies{Database: database}}, ctx
}

// TestAccountResolver_CRUD exercises CreateAccount, Account, UpdateAccount, and DeleteAccount.
func TestAccountResolver_CRUD(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	created, err := mr.CreateAccount(ctx, "api-crud@example.com", "apicrud", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "api-crud@example.com", created.Email)
	assert.Equal(t, "apicrud", created.Username)
	assert.False(t, created.CreatedAt.IsZero())

	fetched, err := qr.Account(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Email, fetched.Email)

	newEmail := "api-updated@example.com"
	updated, err := mr.UpdateAccount(ctx, created.ID, &newEmail, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newEmail, updated.Email)
	assert.Equal(t, "apicrud", updated.Username)

	deleted, err := mr.DeleteAccount(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	gone, err := qr.Account(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

// TestAccountResolver_CreateAccount_GeneratesID when the resolver builds a model without ID.
func TestAccountResolver_CreateAccount_GeneratesID(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	created, err := mr.CreateAccount(ctx, "api-auto@example.com", "apiauto", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.True(t, strings.HasPrefix(created.ID, "account_"))
}

// TestAccountResolver_Account_NotFound returns nil without error.
func TestAccountResolver_Account_NotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	qr := &queryResolver{r}

	none, err := qr.Account(ctx, "account_does_not_exist")
	require.NoError(t, err)
	assert.Nil(t, none)
}

// TestAccountResolver_DeleteAccount_NotFound returns false without error.
func TestAccountResolver_DeleteAccount_NotFound(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	ok, err := mr.DeleteAccount(ctx, "account_never_created")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestAccountResolver_UpdateAccount_ValidationError propagates from the database layer.
func TestAccountResolver_UpdateAccount_ValidationError(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	_, err := mr.UpdateAccount(ctx, "any_id", nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of email, username, teamId, clearTeamId, or role")
}

// TestResolver_createAccount_getAccount_updateAccount_deleteAccount covers the thin helpers on *Resolver.
func TestResolver_createAccount_getAccount_updateAccount_deleteAccount(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)

	acc, err := r.createAccount(ctx, "helper@example.com", "helperuser", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, acc)

	got, err := r.getAccount(ctx, acc.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, acc.ID, got.ID)

	email := "helper2@example.com"
	upd, err := r.updateAccount(ctx, acc.ID, &email, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, upd)
	assert.Equal(t, email, upd.Email)

	ok, err := r.deleteAccount(ctx, acc.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	final, err := r.getAccount(ctx, acc.ID)
	require.NoError(t, err)
	assert.Nil(t, final)
}

func TestAccountResolver_Accounts(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	empty, err := qr.Accounts(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	assert.Empty(t, empty.Items)
	assert.Zero(t, empty.TotalCount)

	team, err := mr.CreateTeam(ctx, "resolver-accounts-team")
	require.NoError(t, err)

	_, err = mr.CreateAccount(ctx, "solo@example.com", "solo", nil, nil)
	require.NoError(t, err)
	_, err = mr.CreateAccount(ctx, "member@example.com", "member", &team.ID, nil)
	require.NoError(t, err)

	all, err := qr.Accounts(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, all.Items, 2)
	assert.Equal(t, 2, all.TotalCount)
	assert.False(t, all.HasNextPage)
	assert.Equal(t, "member", all.Items[0].Username)
	assert.Equal(t, "solo", all.Items[1].Username)

	byTeam, err := qr.Accounts(ctx, &team.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, byTeam.Items, 1)
	assert.Equal(t, 1, byTeam.TotalCount, "totalCount must respect the team filter")
	assert.Equal(t, "member", byTeam.Items[0].Username)
}

func TestAccountResolver_Accounts_Pagination(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	for _, name := range []string{"one", "two", "three"} {
		_, err := mr.CreateAccount(ctx, name+"@example.com", name, nil, nil)
		require.NoError(t, err)
	}

	limit, offset := 2, 0
	first, err := qr.Accounts(ctx, nil, &limit, &offset)
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	assert.Equal(t, 3, first.TotalCount)
	assert.True(t, first.HasNextPage)
	assert.Equal(t, []string{"three", "two"}, []string{first.Items[0].Username, first.Items[1].Username})

	offset = 2
	last, err := qr.Accounts(ctx, nil, &limit, &offset)
	require.NoError(t, err)
	require.Len(t, last.Items, 1)
	assert.False(t, last.HasNextPage)
	assert.Equal(t, "one", last.Items[0].Username)
}

func TestAccountResolver_Accounts_ClampsLimit(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	_, err := mr.CreateAccount(ctx, "only@example.com", "only", nil, nil)
	require.NoError(t, err)

	// An oversized limit is clamped to MaxPageLimit rather than rejected.
	huge := MaxPageLimit * 10
	page, err := qr.Accounts(ctx, nil, &huge, nil)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, 1, page.TotalCount)
	assert.False(t, page.HasNextPage)
}

// asPrincipal returns a copy of ctx authenticated as accountID with the
// given role, scoped to teamID ("" for an unaffiliated principal).
func asPrincipal(ctx context.Context, accountID string, role model.Role, teamID string) context.Context {
	principal := &auth.Principal[model.Identity, model.Role]{
		ID:    accountID,
		Type:  model.IdentityAccount,
		OrgID: teamID,
	}
	return auth.WithPrincipal(ctx, principal.WithRoles(role))
}

// TestAccountResolver_UpdateAccount_RoleChange_DeniedForMember asserts the
// business rule that only OWNER/ADMIN callers may change another account's role.
func TestAccountResolver_UpdateAccount_RoleChange_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	target, err := mr.CreateAccount(ctx, "target@example.com", "target", nil, nil)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "caller@example.com", "caller", nil, nil)
	require.NoError(t, err)

	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	admin := model.RoleAdmin
	_, err = mr.UpdateAccount(memberCtx, target.ID, nil, nil, nil, nil, &admin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	// The role must be unchanged.
	unchanged, err := (&queryResolver{r}).Account(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, model.RoleMember, unchanged.Role)
}

// TestAccountResolver_UpdateAccount_RoleChange_AllowedForAdmin is the converse:
// an OWNER/ADMIN caller may change another account's role.
func TestAccountResolver_UpdateAccount_RoleChange_AllowedForAdmin(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	target, err := mr.CreateAccount(ctx, "target2@example.com", "target2", nil, nil)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "caller2@example.com", "caller2", nil, nil)
	require.NoError(t, err)

	adminCtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, "")

	admin := model.RoleAdmin
	updated, err := mr.UpdateAccount(adminCtx, target.ID, nil, nil, nil, nil, &admin)
	require.NoError(t, err)
	assert.Equal(t, model.RoleAdmin, updated.Role)
}

// TestAccountResolver_UpdateAccount_RoleChange_DeniedForAdminOfAnotherTeam
// asserts the strict-per-team RBAC rule: an ADMIN of one team cannot change
// the role of an account belonging to a different team.
func TestAccountResolver_UpdateAccount_RoleChange_DeniedForAdminOfAnotherTeam(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	teamA, err := mr.CreateTeam(ctx, "role-change-team-a")
	require.NoError(t, err)
	teamB, err := mr.CreateTeam(ctx, "role-change-team-b")
	require.NoError(t, err)

	target, err := mr.CreateAccount(ctx, "target-in-b@example.com", "targetinb", &teamB.ID, nil)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "admin-of-a-for-role@example.com", "adminofaforrole", &teamA.ID, nil)
	require.NoError(t, err)
	adminOfACtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, teamA.ID)

	admin := model.RoleAdmin
	_, err = mr.UpdateAccount(adminOfACtx, target.ID, nil, nil, nil, nil, &admin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

// TestAccountResolver_DeleteAccount_DeniedForMember asserts the business rule
// that only OWNER/ADMIN callers may delete accounts.
func TestAccountResolver_DeleteAccount_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	target, err := mr.CreateAccount(ctx, "todelete@example.com", "todelete", nil, nil)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "deleter@example.com", "deleter", nil, nil)
	require.NoError(t, err)

	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	_, err = mr.DeleteAccount(memberCtx, target.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	still, err := (&queryResolver{r}).Account(ctx, target.ID)
	require.NoError(t, err)
	assert.NotNil(t, still)
}

// TestAccountResolver_DeleteAccount_DeniedForAdminOfAnotherTeam asserts the
// strict-per-team RBAC rule for deletion: an ADMIN of one team cannot
// delete an account belonging to a different team.
func TestAccountResolver_DeleteAccount_DeniedForAdminOfAnotherTeam(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	teamA, err := mr.CreateTeam(ctx, "delete-team-a")
	require.NoError(t, err)
	teamB, err := mr.CreateTeam(ctx, "delete-team-b")
	require.NoError(t, err)

	target, err := mr.CreateAccount(ctx, "target-to-delete-in-b@example.com", "targettodeleteinb", &teamB.ID, nil)
	require.NoError(t, err)

	caller, err := mr.CreateAccount(ctx, "admin-of-a-for-delete@example.com", "adminofafordelete", &teamA.ID, nil)
	require.NoError(t, err)
	adminOfACtx := asPrincipal(ctx, caller.ID, model.RoleAdmin, teamA.ID)

	_, err = mr.DeleteAccount(adminOfACtx, target.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	still, err := (&queryResolver{r}).Account(ctx, target.ID)
	require.NoError(t, err)
	assert.NotNil(t, still)
}

// TestAccountResolver_CreateAccount_ElevatedRole_DeniedForMember asserts that
// only OWNER/ADMIN callers may create an account with a role other than the
// MEMBER default.
func TestAccountResolver_CreateAccount_ElevatedRole_DeniedForMember(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}

	caller, err := mr.CreateAccount(ctx, "creator@example.com", "creator", nil, nil)
	require.NoError(t, err)

	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	admin := model.RoleAdmin
	_, err = mr.CreateAccount(memberCtx, "elevated@example.com", "elevated", nil, &admin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

// TestAccountResolver_Me returns the account backing the caller's own principal,
// which is how the frontend knows what the current user is allowed to do.
func TestAccountResolver_Me(t *testing.T) {
	t.Parallel()

	r, ctx := testResolver(t)
	mr := &mutationResolver{r}
	qr := &queryResolver{r}

	caller, err := mr.CreateAccount(ctx, "whoami@example.com", "whoami", nil, nil)
	require.NoError(t, err)

	memberCtx := asPrincipal(ctx, caller.ID, model.RoleMember, "")

	me, err := qr.Me(memberCtx)
	require.NoError(t, err)
	require.NotNil(t, me)
	assert.Equal(t, caller.ID, me.ID)
	assert.Equal(t, model.RoleMember, me.Role)
}

// TestAccountResolver_Me_NoPrincipal errors when there's no authenticated principal.
func TestAccountResolver_Me_NoPrincipal(t *testing.T) {
	t.Parallel()

	r, _ := testResolver(t)
	qr := &queryResolver{r}

	_, err := qr.Me(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication required")
}
