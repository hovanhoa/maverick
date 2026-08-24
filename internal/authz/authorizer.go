// Package authz implements pkg/core/http.Authorizer for this project.
// Authentication accepts two credential forms, tried in the order below:
// a permanent API key (the sole mechanism for the LLM proxy, and available
// for the management API too), and a short-lived session JWT minted by
// POST /login for the web console.
package authz

import (
	"context"
	"strings"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// apiKeyPrefix identifies a token as an API key rather than a session JWT -
// every API key is minted via encoding.NewRandomIdentifierWithLength("llmgw",
// ...), and a JWT (three dot-separated base64url segments) can never start
// with it.
const apiKeyPrefix = "llmgw_"

// Dependencies of the Authorizer.
type Dependencies struct {
	Database *db.Database

	// Tokens verifies the session JWTs minted by POST /login. Nil disables
	// session-token authentication entirely - only API keys are accepted.
	Tokens *auth.TokenService[model.Identity]
}

// Authorizer resolves principals from API keys and session JWTs.
type Authorizer struct {
	deps Dependencies
}

// New returns a new Authorizer with the given dependencies.
func New(deps Dependencies) *Authorizer {
	return &Authorizer{deps: deps}
}

// GetPrincipalFromToken resolves a principal from either an API key or a
// session JWT. A token bearing the API key prefix is always treated as an
// API key; anything else is tried as a session JWT first (when Tokens is
// configured), falling back to the API key lookup so a malformed or expired
// session JWT still fails with the same "invalid or revoked API key"
// message rather than a JWT-specific one.
func (a *Authorizer) GetPrincipalFromToken(ctx context.Context, token auth.TokenString, namespace string) (*auth.Principal[model.Identity, model.Role], error) {
	if a.deps.Tokens != nil && !strings.HasPrefix(string(token), apiKeyPrefix) {
		if principal, err := a.principalFromSessionToken(ctx, token); err == nil {
			return principal, nil
		}
	}
	return a.principalFromAPIKey(ctx, token)
}

// principalFromAPIKey hashes the token and looks it up against issued,
// non-revoked API keys.
func (a *Authorizer) principalFromAPIKey(ctx context.Context, token auth.TokenString) (*auth.Principal[model.Identity, model.Role], error) {
	account, apiKey, err := a.deps.Database.GetAccountByAPIKeyHash(ctx, db.HashAPIKey(string(token)))
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("invalid or revoked API key")
	}
	return model.WithAPIKeyID(principalForAccount(account), apiKey.ID), nil
}

// principalFromSessionToken verifies the token as a session JWT and
// re-fetches the account it names, so a role or team change since login
// takes effect without requiring the caller to sign in again.
func (a *Authorizer) principalFromSessionToken(ctx context.Context, token auth.TokenString) (*auth.Principal[model.Identity, model.Role], error) {
	claims, err := a.deps.Tokens.VerifyJWT(ctx, token)
	if err != nil {
		return nil, err
	}

	account, err := a.deps.Database.GetAccountByID(ctx, claims.Subject)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("account not found")
	}

	return principalForAccount(account), nil
}

func principalForAccount(account *model.Account) *auth.Principal[model.Identity, model.Role] {
	principal := &auth.Principal[model.Identity, model.Role]{
		ID:    account.ID,
		Type:  model.IdentityAccount,
		Email: account.Email,
		Name:  account.Username,
	}
	if account.TeamID != nil {
		principal.OrgID = *account.TeamID
	}

	return principal.WithRoles(account.Role)
}

// GetPrincipalFromEmail is not applicable to this project: API keys are the
// only authentication mechanism, for both the management API and the
// proxy. There is no separate email/SSO based fallback.
func (a *Authorizer) GetPrincipalFromEmail(ctx context.Context, email string, namespace string) (*auth.Principal[model.Identity, model.Role], error) {
	return nil, nil
}
