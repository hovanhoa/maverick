package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

func (r *Resolver) createAccount(ctx context.Context, email string, username string, teamID *string, role *model.Role) (*model.Account, error) {
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
	return r.deps.Database.CreateAccount(ctx, account)
}

func (r *Resolver) listAccounts(ctx context.Context, teamID *string, limit *int, offset *int) (*model.AccountConnection, error) {
	resolvedLimit, resolvedOffset, err := resolvePage(limit, offset)
	if err != nil {
		return nil, err
	}

	accounts, total, err := r.deps.Database.ListAccounts(ctx, teamID, resolvedLimit, resolvedOffset)
	if err != nil {
		return nil, err
	}

	return &model.AccountConnection{
		Items:       accounts,
		TotalCount:  total,
		HasNextPage: hasNextPage(resolvedOffset, len(accounts), total),
	}, nil
}

func (r *Resolver) getAccount(ctx context.Context, id string) (*model.Account, error) {
	return r.deps.Database.GetAccountByID(ctx, id)
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

func (r *Resolver) updateAccount(ctx context.Context, id string, email *string, username *string, teamID *string, clearTeamID *bool, role *model.Role) (*model.Account, error) {
	if role != nil {
		// Changing someone's role is scoped by their *current* team - an
		// OWNER/ADMIN may only do this for members of their own team.
		if err := requireOwnerOrAdminOfAccountsTeam(ctx, r, id); err != nil {
			return nil, err
		}
	}
	return r.deps.Database.UpdateAccount(ctx, id, email, username, teamID, clearTeamID, role)
}

func (r *Resolver) deleteAccount(ctx context.Context, id string) (bool, error) {
	if err := requireOwnerOrAdminOfAccountsTeam(ctx, r, id); err != nil {
		return false, err
	}
	return r.deps.Database.DeleteAccount(ctx, id)
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
