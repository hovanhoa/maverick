package http

import (
	"net/http"
	"strings"

	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// ExtractJWTFromRequest attempts to extract a JWT token from an HTTP request.
// It tries the following sources in order:
// 1. JWT cookie (using the application namespace)
// 2. Authorization header (Bearer token)
// Returns the token string if found, or an error if not found or invalid.
func ExtractJWTFromRequest(r *http.Request, namespace string) (auth.TokenString, error) {
	// Try to get JWT from cookie first
	token, err := ExtractJWTFromCookie(r, namespace)
	if err == nil && token != "" {
		return token, nil
	}

	// Fall back to Bearer token in Authorization header
	token, err = ExtractBearerToken(r)
	if err == nil && token != "" {
		return token, nil
	}

	return "", errors.New("no JWT token found in request")
}

// ExtractJWTFromCookie extracts a JWT token from the session cookie.
// The cookie name is determined by the application namespace.
func ExtractJWTFromCookie(r *http.Request, namespace string) (auth.TokenString, error) {
	cookieName := GetSessionJWTCookieID(namespace)
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return "", errors.Wrap(err, "cookie not found: %s", cookieName)
	}

	if cookie.Value == "" {
		return "", errors.New("empty JWT cookie value")
	}

	return auth.TokenString(cookie.Value), nil
}

// ExtractBearerToken extracts a JWT token from the Authorization header.
// Expected format: "Authorization: Bearer <token>"
func ExtractBearerToken(r *http.Request) (auth.TokenString, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("no Authorization header")
	}

	// Check for Bearer token format
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid Authorization header format (expected 'Bearer <token>')")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty bearer token")
	}

	return auth.TokenString(token), nil
}

// ExtractEmailFromRequest extracts an email address from the request.
// The email address is extracted from the X-Token-User-Email header.
func ExtractEmailFromRequest(r *http.Request) (string, error) {
	// Try headers in order to get the first non-empty email address
	tryHeaders := []string{
		"X-Token-User-Email",
		"X-Oyster-Retool-User",
	}
	for _, header := range tryHeaders {
		email := r.Header.Get(header)
		if email != "" {
			return email, nil
		}
	}

	return "", errors.New("no email address found in request headers")
}
