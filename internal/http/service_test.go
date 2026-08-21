package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/stretchr/testify/require"
)

const graphqlAccountsQuery = `{"query":"{ accounts { totalCount } }"}`

// TestGraphQL_AuthMiddleware exercises AuthMiddleware/RequireAuth as wired
// ahead of /graphql: a valid, non-revoked API key must resolve to the
// correct principal and be let through, while a missing, invalid, or
// revoked key must be rejected with 401.
func TestGraphQL_AuthMiddleware(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{
		Email:    "middleware@example.com",
		Username: "middlewareuser",
		Role:     model.RoleAdmin,
	})
	require.NoError(t, err)

	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	revokedSecret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)
	revoked, err := database.RevokeAPIKey(ctx, revokedSecret.APIKey.ID)
	require.NoError(t, err)
	require.True(t, revoked)

	service := NewService(Dependencies{DB: database, Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	t.Run("valid key is authenticated", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/graphql/query").
			WithHeader("Authorization", "Bearer "+secret.Key).
			WithBodyString(graphqlAccountsQuery).
			WithHeader("Content-Type", "application/json").
			Build()

		tester.Run(req).AssertStatusCode(http.StatusOK)
	})

	t.Run("missing key is rejected", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/graphql/query").
			WithBodyString(graphqlAccountsQuery).
			WithHeader("Content-Type", "application/json").
			Build()

		tester.Run(req).AssertStatusCode(http.StatusUnauthorized)
	})

	t.Run("invalid key is rejected", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/graphql/query").
			WithHeader("Authorization", "Bearer not-a-real-key").
			WithBodyString(graphqlAccountsQuery).
			WithHeader("Content-Type", "application/json").
			Build()

		tester.Run(req).AssertStatusCode(http.StatusUnauthorized)
	})

	t.Run("revoked key is rejected", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/graphql/query").
			WithHeader("Authorization", "Bearer "+revokedSecret.Key).
			WithBodyString(graphqlAccountsQuery).
			WithHeader("Content-Type", "application/json").
			Build()

		tester.Run(req).AssertStatusCode(http.StatusUnauthorized)
	})
}
