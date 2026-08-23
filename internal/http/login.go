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

// LoginResponse hands back an API key on successful login, which the caller
// then uses exactly like any other API key - login is just a friendlier way
// to obtain one than pasting a pre-issued key. Each successful login mints a
// fresh key (plaintext API keys are never stored, so an existing key can't
// be re-handed-out); old keys from previous logins stay valid until revoked
// from the API Keys page.
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

	secret, err := s.deps.DB.CreateAPIKey(ctx, account.ID)
	if err != nil {
		return nil, http.FromError(err)
	}

	return http.HandlerResponseJSON(LoginResponse{
		Key:     secret.Key,
		Account: *account,
	}), nil
}
