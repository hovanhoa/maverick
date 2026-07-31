package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
)

func (r *Resolver) createAccount(ctx context.Context, email string, username string, teamID *string) (*model.Account, error) {
	account := &model.Account{
		Email:    email,
		Username: username,
		TeamID:   teamID,
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

func (r *Resolver) updateAccount(ctx context.Context, id string, email *string, username *string, teamID *string, clearTeamID *bool) (*model.Account, error) {
	return r.deps.Database.UpdateAccount(ctx, id, email, username, teamID, clearTeamID)
}

func (r *Resolver) deleteAccount(ctx context.Context, id string) (bool, error) {
	return r.deps.Database.DeleteAccount(ctx, id)
}
