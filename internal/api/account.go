package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
)

func (r *Resolver) createAccount(ctx context.Context, email string, username string) (*model.Account, error) {
	account := &model.Account{
		Email:    email,
		Username: username,
	}
	return r.deps.Database.CreateAccount(ctx, account)
}

func (r *Resolver) getAccount(ctx context.Context, id string) (*model.Account, error) {
	return r.deps.Database.GetAccountByID(ctx, id)
}
