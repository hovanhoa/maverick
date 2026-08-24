package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

func (r *Resolver) createAccount(ctx context.Context, email string, username string, teamID *string, role *model.Role) (*model.AccountSecret, error) {
	resolvedRole := model.RoleMember
	if role != nil {
		resolvedRole = *role
	}
	if resolvedRole != model.RoleMember {
		// teamID is the team the new account is being created into, so an
		// OWNER/ADMIN may only do this for their own team. An unaffiliated
		// creation (teamID == nil) has no team to scope by, matching
		// createTeam's bootstrap-time behavior.
		if teamID != nil {
			if err := requireTeamRole(ctx, *teamID, model.RoleOwner, model.RoleAdmin); err != nil {
				return nil, err
			}
		} else if err := requireRole(ctx, model.RoleOwner, model.RoleAdmin); err != nil {
			return nil, err
		}
	}

	account := &model.Account{
		Email:    email,
		Username: username,
		TeamID:   teamID,
		Role:     resolvedRole,
	}
	created, err := r.deps.Database.CreateAccount(ctx, account)
	if err != nil {
		return nil, err
	}

	password, err := r.deps.Database.SetRandomAccountPassword(ctx, created.ID)
	if err != nil {
		return nil, err
	}

	return &model.AccountSecret{Account: *created, Password: password}, nil
}

// resetAccountPassword generates a fresh random password for an existing
// account, gated the same way deleteAccount is: the caller must be
// OWNER/ADMIN of the target's team (or a platform-wide OWNER/ADMIN for an
// unaffiliated account). There is no self-service path here - a forgotten
// password can only be reset by someone else, same as it can't be recovered
// by the account holder themselves.
func (r *Resolver) resetAccountPassword(ctx context.Context, id string) (*model.AccountSecret, error) {
	if err := requireOwnerOrAdminOfAccountsTeam(ctx, r, id); err != nil {
		return nil, err
	}

	account, err := r.deps.Database.GetAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("account not found")
	}

	password, err := r.deps.Database.SetRandomAccountPassword(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.AccountSecret{Account: *account, Password: password}, nil
}

// listAccounts is team-scoped: an explicit teamId still requires membership
// in that team (requireTeamMember), and omitting it doesn't mean "every
// account on the platform" - there's no platform-admin concept for that to
// mean anything safe - it defaults to the caller's own team's roster. An
// unaffiliated caller has no team roster to default to, but should still
// see themselves rather than a page that omits even their own account.
func (r *Resolver) listAccounts(ctx context.Context, teamID *string, limit *int, offset *int) (*model.AccountConnection, error) {
	resolvedLimit, resolvedOffset, err := resolvePage(limit, offset)
	if err != nil {
		return nil, err
	}

	effectiveTeamID := teamID
	if effectiveTeamID != nil {
		if err := requireTeamMember(ctx, *effectiveTeamID); err != nil {
			return nil, err
		}
	} else {
		principal := currentPrincipal(ctx)
		if principal == nil {
			return &model.AccountConnection{Items: []model.Account{}, TotalCount: 0, HasNextPage: false}, nil
		}
		if principal.OrgID == "" {
			self, err := r.deps.Database.GetAccountByID(ctx, principal.ID)
			if err != nil {
				return nil, err
			}
			if self == nil || resolvedOffset > 0 {
				return &model.AccountConnection{Items: []model.Account{}, TotalCount: 0, HasNextPage: false}, nil
			}
			return &model.AccountConnection{Items: []model.Account{*self}, TotalCount: 1, HasNextPage: false}, nil
		}
		effectiveTeamID = &principal.OrgID
	}

	accounts, total, err := r.deps.Database.ListAccounts(ctx, effectiveTeamID, resolvedLimit, resolvedOffset)
	if err != nil {
		return nil, err
	}

	return &model.AccountConnection{
		Items:       accounts,
		TotalCount:  total,
		HasNextPage: hasNextPage(resolvedOffset, len(accounts), total),
	}, nil
}

// getAccount is readable by the account itself, or by anyone belonging to
// the same team - an unaffiliated target (no team) has no boundary to
// enforce, so it stays readable by any authenticated caller, matching the
// pre-existing behavior for that case.
func (r *Resolver) getAccount(ctx context.Context, id string) (*model.Account, error) {
	if principal := currentPrincipal(ctx); principal != nil && principal.ID == id {
		return r.deps.Database.GetAccountByID(ctx, id)
	}

	account, err := r.deps.Database.GetAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if account == nil || account.TeamID == nil {
		return account, nil
	}
	if err := requireTeamMember(ctx, *account.TeamID); err != nil {
		return nil, err
	}
	return account, nil
}

// getMe returns the account backing the principal authenticated on this
// request. RequireAuth (wired ahead of /graphql) guarantees a principal is
// present here.
func (r *Resolver) getMe(ctx context.Context) (*model.Account, error) {
	principal := currentPrincipal(ctx)
	if principal == nil {
		return nil, errors.New("authentication required")
	}

	account, err := r.deps.Database.GetAccountByID(ctx, principal.ID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("account not found")
	}

	return account, nil
}

func (r *Resolver) updateAccount(ctx context.Context, id string, email *string, username *string, name *string, teamID *string, clearTeamID *bool, role *model.Role) (*model.Account, error) {
	// A caller may always edit their own email/username/name (self-service),
	// but changing role or team membership - for self or anyone else - is
	// a privileged action scoped to the target account's *current* team:
	// an OWNER/ADMIN may only do this for members of their own team.
	// Editing anyone else's account, for any field, needs the same check -
	// there is no bare "logged in" tier of access to another account.
	principal := currentPrincipal(ctx)
	isSelf := principal != nil && principal.ID == id
	changesMembership := role != nil || teamID != nil || (clearTeamID != nil && *clearTeamID)

	if changesMembership || !isSelf {
		if err := requireOwnerOrAdminOfAccountsTeam(ctx, r, id); err != nil {
			return nil, err
		}
	}

	return r.deps.Database.UpdateAccount(ctx, id, email, username, name, teamID, clearTeamID, role)
}

func (r *Resolver) deleteAccount(ctx context.Context, id string) (bool, error) {
	if err := requireOwnerOrAdminOfAccountsTeam(ctx, r, id); err != nil {
		return false, err
	}
	return r.deps.Database.DeleteAccount(ctx, id)
}

func (r *Resolver) updateAccountQuota(ctx context.Context, id string, monthlyTokenBudget *int, clearMonthlyTokenBudget *bool) (*model.Account, error) {
	if err := requireOwnerOrAdminOfAccountsTeam(ctx, r, id); err != nil {
		return nil, err
	}
	return r.deps.Database.UpdateAccountQuota(ctx, id, monthlyTokenBudget, clearMonthlyTokenBudget)
}

// requireOwnerOrAdminOfAccountsTeam requires the caller to be OWNER/ADMIN
// of the target account's current team. An unaffiliated target (no team,
// or not found - DeleteAccount/UpdateAccount on a missing id is a no-op
// handled downstream) falls back to a plain role check, since there's no
// team to scope by.
func requireOwnerOrAdminOfAccountsTeam(ctx context.Context, r *Resolver, accountID string) error {
	target, err := r.deps.Database.GetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	var targetTeamID *string
	if target != nil {
		targetTeamID = target.TeamID
	}
	if targetTeamID != nil {
		return requireTeamRole(ctx, *targetTeamID, model.RoleOwner, model.RoleAdmin)
	}
	return requireRole(ctx, model.RoleOwner, model.RoleAdmin)
}
