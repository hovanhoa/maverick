package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
)

func (r *Resolver) createAPIKey(ctx context.Context, accountID string) (*model.APIKeySecret, error) {
	if err := requireSelfOrRole(ctx, accountID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}
	return r.deps.Database.CreateAPIKey(ctx, accountID)
}

func (r *Resolver) listAPIKeys(ctx context.Context, accountID string) ([]model.APIKey, error) {
	if err := requireSelfOrRole(ctx, accountID, model.RoleOwner, model.RoleAdmin); err != nil {
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
	if err := requireSelfOrRole(ctx, apiKey.AccountID, model.RoleOwner, model.RoleAdmin); err != nil {
		return false, err
	}
	return r.deps.Database.RevokeAPIKey(ctx, id)
}
