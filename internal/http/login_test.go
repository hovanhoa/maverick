package http

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/hovanhoa/llmgateway/pkg/driver/memkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestTokenService returns a TokenService backed by an in-memory KV
// store and a freshly generated signing key, suitable for exercising
// session-JWT login/auth in tests without a real Redis instance.
func newTestTokenService(t *testing.T) *auth.TokenService[model.Identity] {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	return auth.NewTokenService[model.Identity](auth.Dependencies{
		DB:            memkv.New(),
		JWTPrivateKey: privateKey,
		JWTPublicKey:  &privateKey.PublicKey,
	})
}

// TestLogin exercises POST /login: it must work without any prior
// authentication (there's no key yet to authenticate with), succeed with
// the right username/password and hand back a usable API key, and reject
// everything else without revealing which part was wrong.
func TestLogin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{
		Email:    "login@example.com",
		Username: "loginuser",
		Role:     model.RoleMember,
	})
	require.NoError(t, err)

	password, err := database.SetRandomAccountPassword(ctx, account.ID)
	require.NoError(t, err)

	noPasswordAccount, err := database.CreateAccount(ctx, &model.Account{
		Email:    "nopassword@example.com",
		Username: "nopassworduser",
		Role:     model.RoleMember,
	})
	require.NoError(t, err)

	service := NewService(Dependencies{DB: database, Tokens: newTestTokenService(t), Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	t.Run("correct username and password logs in without a prior key", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/login").
			WithBodyJSON(LoginRequest{Username: "loginuser", Password: password}).
			Build()

		resp := tester.Run(req).AssertStatusCode(http.StatusOK)
		body := corehttp.UnmarshalJSONBody[LoginResponse](resp)
		assert.NotEmpty(t, body.Key)
		assert.Equal(t, account.ID, body.Account.ID)

		// The returned key actually authenticates against the rest of the API.
		graphqlReq := corehttp.NewRequestBuilder(http.MethodPost, "/graphql/query").
			WithHeader("Authorization", "Bearer "+body.Key).
			WithHeader("Content-Type", "application/json").
			WithBodyString(`{"query":"{ me { id } }"}`).
			Build()
		tester.Run(graphqlReq).AssertStatusCode(http.StatusOK)
	})

	t.Run("wrong password is rejected", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/login").
			WithBodyJSON(LoginRequest{Username: "loginuser", Password: "not-the-password"}).
			Build()

		tester.Run(req).AssertStatusCode(http.StatusUnauthorized)
	})

	t.Run("unknown username is rejected the same way as a wrong password", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/login").
			WithBodyJSON(LoginRequest{Username: "nobody-by-this-name", Password: "whatever"}).
			Build()

		tester.Run(req).AssertStatusCode(http.StatusUnauthorized)
	})

	t.Run("an account with no password set yet is rejected, not a server error", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/login").
			WithBodyJSON(LoginRequest{Username: noPasswordAccount.Username, Password: "anything"}).
			Build()

		tester.Run(req).AssertStatusCode(http.StatusUnauthorized)
	})

	t.Run("missing fields are a bad request", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/login").
			WithBodyJSON(LoginRequest{Username: "loginuser"}).
			Build()

		tester.Run(req).AssertStatusCode(http.StatusBadRequest)
	})

	t.Run("repeated logins never mint an API key", func(t *testing.T) {
		keysBefore, err := database.ListAPIKeysByAccount(ctx, account.ID)
		require.NoError(t, err)

		for range 3 {
			req := corehttp.NewRequestBuilder(http.MethodPost, "/login").
				WithBodyJSON(LoginRequest{Username: "loginuser", Password: password}).
				Build()
			tester.Run(req).AssertStatusCode(http.StatusOK)
		}

		keysAfter, err := database.ListAPIKeysByAccount(ctx, account.ID)
		require.NoError(t, err)
		assert.Len(t, keysAfter, len(keysBefore), "login must authenticate via a session token, not by minting a fresh api_key row every time")
	})
}
