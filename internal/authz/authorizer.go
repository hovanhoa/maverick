// Package authz implements pkg/core/http.Authorizer for this project: an
// API key is the sole authentication mechanism, for both the management
// API (Phase 1) and, from Phase 2 onward, the LLM proxy itself.
package authz

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// Dependencies of the Authorizer.
type Dependencies struct {
	Database *db.Database
}

// Authorizer resolves principals from API keys.
type Authorizer struct {
	deps Dependencies
}

// New returns a new Authorizer with the given dependencies.
func New(deps Dependencies) *Authorizer {
	return &Authorizer{deps: deps}
}

// GetPrincipalFromToken treats the token as an API key: it is hashed and
// looked up against issued, non-revoked keys, and the resulting account is
// converted into a Principal carrying that account's role.
func (a *Authorizer) GetPrincipalFromToken(ctx context.Context, token auth.TokenString, namespace string) (*auth.Principal[model.Identity, model.Role], error) {
	account, err := a.deps.Database.GetAccountByAPIKeyHash(ctx, db.HashAPIKey(string(token)))
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("invalid or revoked API key")
	}

	principal := &auth.Principal[model.Identity, model.Role]{
		ID:    account.ID,
		Type:  model.IdentityAccount,
		Email: account.Email,
		Name:  account.Username,
	}
	if account.TeamID != nil {
		principal.OrgID = *account.TeamID
	}

	return principal.WithRoles(account.Role), nil
}

// GetPrincipalFromEmail is not applicable to this project: API keys are the
// only authentication mechanism, for both the management API and the
// proxy. There is no separate email/SSO based fallback.
func (a *Authorizer) GetPrincipalFromEmail(ctx context.Context, email string, namespace string) (*auth.Principal[model.Identity, model.Role], error) {
	return nil, nil
}
