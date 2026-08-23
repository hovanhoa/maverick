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

// TestAvatar exercises the avatar REST routes end to end: uploading is
// self-service only, GET is public (no Authorization header, since <img>
// tags can't send one), and a missing avatar is a 404 rather than an error.
func TestAvatar(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	owner, err := database.CreateAccount(ctx, &model.Account{Email: "avatar-owner@example.com", Username: "avatarowner"})
	require.NoError(t, err)
	ownerKey, err := database.CreateAPIKey(ctx, owner.ID)
	require.NoError(t, err)

	other, err := database.CreateAccount(ctx, &model.Account{Email: "avatar-other@example.com", Username: "avatarother"})
	require.NoError(t, err)
	otherKey, err := database.CreateAPIKey(ctx, other.ID)
	require.NoError(t, err)

	service := NewService(Dependencies{DB: database, Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	t.Run("no avatar yet is a 404, not an error", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodGet, "/accounts/"+owner.ID+"/avatar").Build()
		tester.Run(req).AssertStatusCode(http.StatusNotFound)
	})

	t.Run("uploading someone else's avatar is forbidden", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/accounts/"+owner.ID+"/avatar").
			WithHeader("Authorization", "Bearer "+otherKey.Key).
			WithHeader("Content-Type", "image/png").
			WithBodyBytes([]byte{0x89, 0x50, 0x4e, 0x47}).
			Build()
		tester.Run(req).AssertStatusCode(http.StatusForbidden)
	})

	t.Run("an unsupported content type is rejected", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodPost, "/accounts/"+owner.ID+"/avatar").
			WithHeader("Authorization", "Bearer "+ownerKey.Key).
			WithHeader("Content-Type", "application/pdf").
			WithBodyBytes([]byte("not an image")).
			Build()
		tester.Run(req).AssertStatusCode(http.StatusBadRequest)
	})

	t.Run("uploading your own avatar succeeds and GET serves it back unauthenticated", func(t *testing.T) {
		imageBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a}
		uploadReq := corehttp.NewRequestBuilder(http.MethodPost, "/accounts/"+owner.ID+"/avatar").
			WithHeader("Authorization", "Bearer "+ownerKey.Key).
			WithHeader("Content-Type", "image/png").
			WithBodyBytes(imageBytes).
			Build()
		tester.Run(uploadReq).AssertStatusCode(http.StatusNoContent)

		getReq := corehttp.NewRequestBuilder(http.MethodGet, "/accounts/"+owner.ID+"/avatar").Build()
		tester.Run(getReq).AssertStatusCode(http.StatusOK).AssertHeader("Content-Type", "image/png")
	})

	t.Run("deleting someone else's avatar is forbidden", func(t *testing.T) {
		req := corehttp.NewRequestBuilder(http.MethodDelete, "/accounts/"+owner.ID+"/avatar").
			WithHeader("Authorization", "Bearer "+otherKey.Key).
			Build()
		tester.Run(req).AssertStatusCode(http.StatusForbidden)
	})

	t.Run("deleting your own avatar removes it", func(t *testing.T) {
		delReq := corehttp.NewRequestBuilder(http.MethodDelete, "/accounts/"+owner.ID+"/avatar").
			WithHeader("Authorization", "Bearer "+ownerKey.Key).
			Build()
		tester.Run(delReq).AssertStatusCode(http.StatusNoContent)

		getReq := corehttp.NewRequestBuilder(http.MethodGet, "/accounts/"+owner.ID+"/avatar").Build()
		tester.Run(getReq).AssertStatusCode(http.StatusNotFound)
	})
}
