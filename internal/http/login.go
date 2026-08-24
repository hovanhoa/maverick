package http

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
)

// LoginRequest is the body of POST /login.
type LoginRequest struct {
	Username string `json:"username" body:"username"`
	Password string `json:"password" body:"password"`
}

// LoginResponse hands back a session token on successful login, which the
// caller then uses exactly like an API key - as an "Authorization: Bearer"
// value against the GraphQL/proxy API. Unlike an API key, this is a
// short-lived, revocable JWT (see pkg/core/auth.TokenService and
// internal/authz.Authorizer, which accepts either credential form) rather
// than a permanent api_key row - a login no longer mints a new API key.
type LoginResponse struct {
	Key     string        `json:"key"`
	Account model.Account `json:"account"`
}

// login implements POST /login. Deliberately not a GraphQL mutation: the
// entire /graphql route requires an existing API key (a single route can't
// be gated per-operation), which makes it unusable for the one request that
// has to work before the caller has a key at all.
func (s *Service) login(ctx context.Context, req *http.HandlerRequest[LoginRequest]) (*http.HandlerResponse, *http.Error) {
	data := req.Data()
	if data.Username == "" || data.Password == "" {
		return nil, http.NewError(http.StatusBadRequest, "username and password are required")
	}

	account, err := s.deps.DB.VerifyAccountPassword(ctx, data.Username, data.Password)
	if err != nil {
		return nil, http.FromError(err)
	}
	if account == nil {
		return nil, http.NewError(http.StatusUnauthorized, "invalid username or password")
	}

	var teamID string
	if account.TeamID != nil {
		teamID = *account.TeamID
	}

	token, err := s.deps.Tokens.GenerateJWT(ctx, account.ID, account.Email, account.Username, teamID, "", model.IdentityAccount)
	if err != nil {
		return nil, http.FromError(err)
	}

	return http.HandlerResponseJSON(LoginResponse{
		Key:     string(token.Token),
		Account: *account,
	}), nil
}
