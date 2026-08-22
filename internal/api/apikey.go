package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
)

// accountsTeamID looks up accountID's current team, for scoping an
// OWNER/ADMIN's access to it. Returns nil (unaffiliated) if the account
// doesn't exist - callers that need "not found" behavior check that
// separately.
func (r *Resolver) accountsTeamID(ctx context.Context, accountID string) (*string, error) {
	account, err := r.deps.Database.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, nil
	}
	return account.TeamID, nil
}

func (r *Resolver) createAPIKey(ctx context.Context, accountID string) (*model.APIKeySecret, error) {
	teamID, err := r.accountsTeamID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := requireSelfOrTeamRole(ctx, accountID, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}
	return r.deps.Database.CreateAPIKey(ctx, accountID)
}

func (r *Resolver) listAPIKeys(ctx context.Context, accountID string) ([]model.APIKey, error) {
	teamID, err := r.accountsTeamID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := requireSelfOrTeamRole(ctx, accountID, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}
	return r.deps.Database.ListAPIKeysByAccount(ctx, accountID)
}

func (r *Resolver) revokeAPIKey(ctx context.Context, id string) (bool, error) {
	apiKey, err := r.deps.Database.GetAPIKeyByID(ctx, id)
	if err != nil {
		return false, err
	}
	if apiKey == nil {
		return false, nil
	}
	teamID, err := r.accountsTeamID(ctx, apiKey.AccountID)
	if err != nil {
		return false, err
	}
	if err := requireSelfOrTeamRole(ctx, apiKey.AccountID, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return false, err
	}
	return r.deps.Database.RevokeAPIKey(ctx, id)
}
