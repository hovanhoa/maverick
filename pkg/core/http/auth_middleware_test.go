package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hovanhoa/llmgateway/pkg/core/auth"
)

// mockAuthorizer is a mock implementation of Authorizer for testing
type mockAuthorizer struct {
	tokenPrincipal *auth.Principal[string, string]
	tokenErr       error
	emailPrincipal *auth.Principal[string, string]
	emailErr       error

	// Captured arguments
	lastTokenReceived  auth.TokenString
	lastTokenNamespace string
	lastEmailReceived  string
	lastEmailNamespace string
	tokenCallCount     int
	emailCallCount     int
}

func (m *mockAuthorizer) GetPrincipalFromToken(ctx context.Context, token auth.TokenString, namespace string) (*auth.Principal[string, string], error) {
	m.lastTokenReceived = token
	m.lastTokenNamespace = namespace
	m.tokenCallCount++
	return m.tokenPrincipal, m.tokenErr
}

func (m *mockAuthorizer) GetPrincipalFromEmail(ctx context.Context, email string, namespace string) (*auth.Principal[string, string], error) {
	m.lastEmailReceived = email
	m.lastEmailNamespace = namespace
	m.emailCallCount++
	return m.emailPrincipal, m.emailErr
}

// createTestPrincipal is a helper that creates a test principal
func createTestPrincipal(id, principalType, email string) *auth.Principal[string, string] {
	return &auth.Principal[string, string]{
		ID:      id,
		Type:    principalType,
		Email:   email,
		Name:    "Test User",
		OrgID:   "org_123",
		OrgName: "Test Org",
	}
}

func TestPrincipalRequestLogFields(t *testing.T) {
	principal := createTestPrincipal("user_123", "user", "user@example.com")

	assert.Equal(t, map[string]any{
		"id":      principal.ID,
		"type":    principal.Type,
		"email":   principal.Email,
		"name":    principal.Name,
		"orgId":   principal.OrgID,
		"orgName": principal.OrgName,
	}, principalRequestLogFields(principal))
}

// setupMiddlewareTest is a helper that sets up a test context for middleware testing
func setupMiddlewareTest(t *testing.T) (*Context, *httptest.ResponseRecorder) {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := NewTestContext(w, req)

	return c, w
}

func TestAuthMiddleware_WithValidJWTInCookie_StoresPrincipal(t *testing.T) {
	namespace := "test-app"
	expectedToken := "valid-jwt-token"
	expectedPrincipal := createTestPrincipal("user_123", "user", "user@example.com")

	authorizer := &mockAuthorizer{
		tokenPrincipal: expectedPrincipal,
		tokenErr:       nil,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(namespace),
		Value: expectedToken,
	})
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.tokenCallCount, "GetPrincipalFromToken should be called once")
	assert.Equal(t, auth.TokenString(expectedToken), authorizer.lastTokenReceived, "Should pass correct token")
	assert.Equal(t, namespace, authorizer.lastTokenNamespace, "Should pass correct namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	require.NotNil(t, principal, "Principal should be stored in context")
	assert.Equal(t, expectedPrincipal.ID, principal.ID)
	assert.Equal(t, expectedPrincipal.Type, principal.Type)
	assert.Equal(t, expectedPrincipal.Email, principal.Email)
}

func TestAuthMiddleware_WithValidBearerToken_StoresPrincipal(t *testing.T) {
	namespace := "test-app"
	expectedToken := "valid-token"
	expectedPrincipal := createTestPrincipal("user_456", "api_key", "api@example.com")

	authorizer := &mockAuthorizer{
		tokenPrincipal: expectedPrincipal,
		tokenErr:       nil,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.Header.Set("Authorization", "Bearer "+expectedToken)
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.tokenCallCount, "GetPrincipalFromToken should be called once")
	assert.Equal(t, auth.TokenString(expectedToken), authorizer.lastTokenReceived, "Should pass correct token")
	assert.Equal(t, namespace, authorizer.lastTokenNamespace, "Should pass correct namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	require.NotNil(t, principal, "Principal should be stored in context")
	assert.Equal(t, expectedPrincipal.ID, principal.ID)
	assert.Equal(t, expectedPrincipal.Type, principal.Type)
	assert.Equal(t, expectedPrincipal.Email, principal.Email)
}

func TestAuthMiddleware_WithValidEmail_StoresPrincipal(t *testing.T) {
	namespace := "test-app"
	expectedEmail := "internal@oysterinc.net"
	expectedPrincipal := createTestPrincipal("user_789", "internal", expectedEmail)

	authorizer := &mockAuthorizer{
		emailPrincipal: expectedPrincipal,
		emailErr:       nil,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.Header.Set("X-Token-User-Email", expectedEmail)
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.emailCallCount, "GetPrincipalFromEmail should be called once")
	assert.Equal(t, expectedEmail, authorizer.lastEmailReceived, "Should pass correct email")
	assert.Equal(t, namespace, authorizer.lastEmailNamespace, "Should pass correct namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	require.NotNil(t, principal, "Principal should be stored in context")
	assert.Equal(t, expectedPrincipal.ID, principal.ID)
	assert.Equal(t, expectedPrincipal.Type, principal.Type)
	assert.Equal(t, expectedPrincipal.Email, principal.Email)
}

func TestAuthMiddleware_WithJWTAndEmail_PrefersJWT(t *testing.T) {
	namespace := "test-app"
	expectedToken := "valid-jwt"
	expectedEmail := "email@example.com"
	jwtPrincipal := createTestPrincipal("jwt_user", "jwt", "jwt@example.com")
	emailPrincipal := createTestPrincipal("email_user", "email", expectedEmail)

	authorizer := &mockAuthorizer{
		tokenPrincipal: jwtPrincipal,
		tokenErr:       nil,
		emailPrincipal: emailPrincipal,
		emailErr:       nil,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(namespace),
		Value: expectedToken,
	})
	c.Request.Header.Set("X-Token-User-Email", expectedEmail)
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.tokenCallCount, "GetPrincipalFromToken should be called once")
	assert.Equal(t, 0, authorizer.emailCallCount, "GetPrincipalFromEmail should not be called when JWT succeeds")
	assert.Equal(t, auth.TokenString(expectedToken), authorizer.lastTokenReceived, "Should pass correct token")
	assert.Equal(t, namespace, authorizer.lastTokenNamespace, "Should pass correct namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	require.NotNil(t, principal, "Principal should be stored in context")
	assert.Equal(t, jwtPrincipal.ID, principal.ID, "Should use JWT principal, not email principal")
}

func TestAuthMiddleware_WithInvalidJWT_FallsBackToEmail(t *testing.T) {
	namespace := "test-app"
	invalidToken := "invalid-jwt"
	expectedEmail := "fallback@example.com"
	emailPrincipal := createTestPrincipal("email_user", "email", expectedEmail)

	authorizer := &mockAuthorizer{
		tokenPrincipal: nil,
		tokenErr:       assert.AnError,
		emailPrincipal: emailPrincipal,
		emailErr:       nil,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(namespace),
		Value: invalidToken,
	})
	c.Request.Header.Set("X-Token-User-Email", expectedEmail)
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.tokenCallCount, "GetPrincipalFromToken should be called first")
	assert.Equal(t, auth.TokenString(invalidToken), authorizer.lastTokenReceived, "Should pass JWT token first")
	assert.Equal(t, 1, authorizer.emailCallCount, "GetPrincipalFromEmail should be called as fallback")
	assert.Equal(t, expectedEmail, authorizer.lastEmailReceived, "Should pass correct email")
	assert.Equal(t, namespace, authorizer.lastEmailNamespace, "Should pass correct namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	require.NotNil(t, principal, "Principal should be stored from email fallback")
	assert.Equal(t, emailPrincipal.ID, principal.ID)
}

func TestAuthMiddleware_WithNoAuthentication_ContinuesWithoutPrincipal(t *testing.T) {
	authorizer := &mockAuthorizer{}

	c, _ := setupMiddlewareTest(t)
	c.Request.Header.Set("X-Oyster-Application-Namespace", "test-app")

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 0, authorizer.tokenCallCount, "GetPrincipalFromToken should not be called")
	assert.Equal(t, 0, authorizer.emailCallCount, "GetPrincipalFromEmail should not be called")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	assert.Nil(t, principal, "Principal should not be in context")
}

func TestAuthMiddleware_WithInvalidJWTAndNoEmail_ContinuesWithoutPrincipal(t *testing.T) {
	namespace := "test-app"
	invalidToken := "invalid-token"

	authorizer := &mockAuthorizer{
		tokenPrincipal: nil,
		tokenErr:       assert.AnError,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.Header.Set("Authorization", "Bearer "+invalidToken)
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.tokenCallCount, "GetPrincipalFromToken should be called")
	assert.Equal(t, auth.TokenString(invalidToken), authorizer.lastTokenReceived, "Should pass correct token")
	assert.Equal(t, 0, authorizer.emailCallCount, "GetPrincipalFromEmail should not be called without email header")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	assert.Nil(t, principal, "Principal should not be in context")
}

func TestAuthMiddleware_WithInvalidEmail_ContinuesWithoutPrincipal(t *testing.T) {
	namespace := "test-app"
	invalidEmail := "invalid@example.com"

	authorizer := &mockAuthorizer{
		emailPrincipal: nil,
		emailErr:       assert.AnError,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.Header.Set("X-Token-User-Email", invalidEmail)
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 0, authorizer.tokenCallCount, "GetPrincipalFromToken should not be called without token")
	assert.Equal(t, 1, authorizer.emailCallCount, "GetPrincipalFromEmail should be called")
	assert.Equal(t, invalidEmail, authorizer.lastEmailReceived, "Should pass correct email")
	assert.Equal(t, namespace, authorizer.lastEmailNamespace, "Should pass correct namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	assert.Nil(t, principal, "Principal should not be in context")
}

func TestAuthMiddleware_WithEmptyNamespace_StillProcessesAuth(t *testing.T) {
	expectedToken := "valid-jwt"
	expectedPrincipal := createTestPrincipal("user_999", "user", "user@example.com")

	authorizer := &mockAuthorizer{
		tokenPrincipal: expectedPrincipal,
		tokenErr:       nil,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.AddCookie(&http.Cookie{
		Name:  GetSessionJWTCookieID(""),
		Value: expectedToken,
	})

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.tokenCallCount, "GetPrincipalFromToken should be called")
	assert.Equal(t, auth.TokenString(expectedToken), authorizer.lastTokenReceived, "Should pass correct token")
	assert.Equal(t, "", authorizer.lastTokenNamespace, "Should pass empty namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	require.NotNil(t, principal, "Principal should be stored even with empty namespace")
	assert.Equal(t, expectedPrincipal.ID, principal.ID)
}

func TestAuthMiddleware_WithNilPrincipalFromAuthorizer_ContinuesWithoutPrincipal(t *testing.T) {
	namespace := "test-app"
	validToken := "valid-token"

	authorizer := &mockAuthorizer{
		tokenPrincipal: nil,
		tokenErr:       nil,
	}

	c, _ := setupMiddlewareTest(t)
	c.Request.Header.Set("Authorization", "Bearer "+validToken)
	c.Request.Header.Set("X-Oyster-Application-Namespace", namespace)

	middleware := AuthMiddleware(authorizer)
	middleware(c)

	assert.Equal(t, 1, authorizer.tokenCallCount, "GetPrincipalFromToken should be called")
	assert.Equal(t, auth.TokenString(validToken), authorizer.lastTokenReceived, "Should pass correct token")
	assert.Equal(t, namespace, authorizer.lastTokenNamespace, "Should pass correct namespace")

	principal := auth.GetPrincipal[string, string](c.Request.Context())
	assert.Nil(t, principal, "Principal should not be in context when authorizer returns nil")
}

func TestRequireAuth_WithAuthenticatedUser_CallsNext(t *testing.T) {
	principal := createTestPrincipal("user_123", "user", "user@example.com")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), principal))

	c := NewTestContext(w, req)

	middleware := RequireAuth[string, string]()
	middleware(c)

	assert.False(t, c.IsAborted(), "Request should not be aborted with authenticated user")
}

func TestRequireAuth_WithNoAuthentication_Returns401(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	c := NewTestContext(w, req)

	middleware := RequireAuth[string, string]()
	middleware(c)

	assert.True(t, c.IsAborted(), "Request should be aborted without authentication")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 status")
}
