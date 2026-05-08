package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hovanhoa/llmgateway/pkg/core/auth"
)

func TestExtractJWTFromRequest_WithValidCookie_ReturnsToken(t *testing.T) {
	namespace := "test-app"
	expectedToken := auth.TokenString("valid-jwt-token")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(namespace),
		Value: string(expectedToken),
	})

	token, err := ExtractJWTFromRequest(req, namespace)

	require.NoError(t, err)
	assert.Equal(t, expectedToken, token)
}

func TestExtractJWTFromRequest_WithValidBearerToken_ReturnsToken(t *testing.T) {
	namespace := "test-app"
	expectedToken := auth.TokenString("valid-jwt-token")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+string(expectedToken))

	token, err := ExtractJWTFromRequest(req, namespace)

	require.NoError(t, err)
	assert.Equal(t, expectedToken, token)
}

func TestExtractJWTFromRequest_WithCookieAndBearer_PrefersCookie(t *testing.T) {
	namespace := "test-app"
	cookieToken := auth.TokenString("cookie-token")
	bearerToken := auth.TokenString("bearer-token")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(namespace),
		Value: string(cookieToken),
	})
	req.Header.Set("Authorization", "Bearer "+string(bearerToken))

	token, err := ExtractJWTFromRequest(req, namespace)

	require.NoError(t, err)
	assert.Equal(t, cookieToken, token, "should prefer cookie over Bearer token")
}

func TestExtractJWTFromRequest_WithNoToken_ReturnsError(t *testing.T) {
	namespace := "test-app"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	token, err := ExtractJWTFromRequest(req, namespace)

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "no JWT token found in request")
}

func TestExtractJWTFromCookie_WithMissingCookie_ReturnsError(t *testing.T) {
	namespace := "test-app"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	token, err := ExtractJWTFromCookie(req, namespace)

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "cookie not found")
}

func TestExtractJWTFromCookie_WithEmptyCookieValue_ReturnsError(t *testing.T) {
	namespace := "test-app"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(namespace),
		Value: "",
	})

	token, err := ExtractJWTFromCookie(req, namespace)

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "empty JWT cookie value")
}

func TestExtractJWTFromCookie_WithWrongNamespace_ReturnsError(t *testing.T) {
	correctNamespace := "app1"
	wrongNamespace := "app2"
	expectedToken := auth.TokenString("jwt-token")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(correctNamespace),
		Value: string(expectedToken),
	})

	token, err := ExtractJWTFromCookie(req, wrongNamespace)

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "cookie not found")
}

func TestExtractBearerToken_WithValidToken_ReturnsToken(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		expectedToken auth.TokenString
	}{
		{
			name:          "lowercase bearer",
			authHeader:    "bearer valid-token",
			expectedToken: "valid-token",
		},
		{
			name:          "uppercase bearer",
			authHeader:    "Bearer valid-token",
			expectedToken: "valid-token",
		},
		{
			name:          "mixed case bearer",
			authHeader:    "BeArEr valid-token",
			expectedToken: "valid-token",
		},
		{
			name:          "token with spaces",
			authHeader:    "Bearer   token-with-leading-spaces",
			expectedToken: "token-with-leading-spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.authHeader)

			token, err := ExtractBearerToken(req)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedToken, token)
		})
	}
}

func TestExtractBearerToken_WithMissingHeader_ReturnsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	token, err := ExtractBearerToken(req)

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "no Authorization header")
}

func TestExtractBearerToken_WithInvalidFormat_ReturnsError(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		errMsg     string
	}{
		{
			name:       "missing bearer prefix",
			authHeader: "valid-token",
			errMsg:     "invalid Authorization header format",
		},
		{
			name:       "wrong prefix",
			authHeader: "Basic valid-token",
			errMsg:     "invalid Authorization header format",
		},
		{
			name:       "only bearer",
			authHeader: "Bearer",
			errMsg:     "invalid Authorization header format",
		},
		{
			name:       "bearer with empty token",
			authHeader: "Bearer   ",
			errMsg:     "empty bearer token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.authHeader)

			token, err := ExtractBearerToken(req)

			require.Error(t, err)
			assert.Empty(t, token)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestExtractEmailFromRequest_WithTokenUserEmail_ReturnsEmail(t *testing.T) {
	expectedEmail := "user@example.com"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Token-User-Email", expectedEmail)

	email, err := ExtractEmailFromRequest(req)

	require.NoError(t, err)
	assert.Equal(t, expectedEmail, email)
}

func TestExtractEmailFromRequest_WithBothHeaders_PrefersTokenUserEmail(t *testing.T) {
	tokenEmail := "token@example.com"
	retoolEmail := "retool@example.com"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Token-User-Email", tokenEmail)
	req.Header.Set("X-Oyster-Retool-User", retoolEmail)

	email, err := ExtractEmailFromRequest(req)

	require.NoError(t, err)
	assert.Equal(t, tokenEmail, email, "should prefer X-Token-User-Email over X-Oyster-Retool-User")
}

func TestExtractEmailFromRequest_WithNoHeaders_ReturnsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	email, err := ExtractEmailFromRequest(req)

	require.Error(t, err)
	assert.Empty(t, email)
	assert.Contains(t, err.Error(), "no email address found in request headers")
}

func TestExtractEmailFromRequest_WithOnlyRetoolHeader_ReturnsRetoolEmail(t *testing.T) {
	retoolEmail := "retool@example.com"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Oyster-Retool-User", retoolEmail)

	email, err := ExtractEmailFromRequest(req)

	require.NoError(t, err)
	assert.Equal(t, retoolEmail, email)
}
