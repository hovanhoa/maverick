package http

import (
	"context"
	"net/http"

	"github.com/hovanhoa/llmgateway/pkg/core/auth"
)

// Authorizer is a interface that defines the methods for getting a principal from a token or email.
// This is to inject the authorization mechanism into the auth middleware.
type Authorizer[Identity ~string, Role ~string] interface {
	// GetPrincipalFromToken gets a principal from a token.
	GetPrincipalFromToken(ctx context.Context, token auth.TokenString, namespace string) (*auth.Principal[Identity, Role], error)

	// GetPrincipalFromEmail gets a principal from an email address.
	GetPrincipalFromEmail(ctx context.Context, email string, namespace string) (*auth.Principal[Identity, Role], error)
}

func principalRequestLogFields[Identity ~string, Role ~string](principal *auth.Principal[Identity, Role]) map[string]any {
	return map[string]any{
		"id":      principal.ID,
		"type":    principal.Type,
		"email":   principal.Email,
		"name":    principal.Name,
		"orgId":   principal.OrgID,
		"orgName": principal.OrgName,
	}
}

// AuthMiddleware is HTTP middleware that extracts and verifies authentication credentials,
// then stores the resulting Principal in the request context.
//
// This middleware:
// 1. Attempts to extract JWT from cookies or Authorization header
// 2. Verifies the JWT using the TokenService
// 3. Converts verified claims to a Principal
// 4. Stores the Principal in the request context
//
// If authentication fails or no credentials are found, the request continues
// without a principal (nil). Use RequireAuth() or RequireRole() middleware
// to enforce authentication requirements.
func AuthMiddleware[Identity ~string, Role ~string](authorizer Authorizer[Identity, Role]) HandlerFunc {
	return func(c *Context) {
		// Get the application namespace from the request header
		namespace := c.Request.Header.Get("X-Oyster-Application-Namespace")

		// Try to extract JWT from request
		token, err := ExtractJWTFromRequest(c.Request, namespace)
		if err == nil && token != "" {
			// Verify the JWT
			principal, err := authorizer.GetPrincipalFromToken(c.Request.Context(), token, namespace)
			if err == nil && principal != nil {
				// Convert claims to principal and store in context
				c.Request = c.Request.WithContext(
					auth.WithPrincipal(c.Request.Context(), principal),
				)

				// Add principal to request logs
				SetRequestLogExtraFields(c.Request.Context(), map[string]any{
					"principal": principalRequestLogFields(principal),
				})

				// JWT authentication succeeded, skip email-based fallback
				c.Next()
				return
			}
		}

		// Only try email-based auth if JWT auth didn't succeed
		// This is a fallback for internal Cara users accessing via *.oysterinc.net
		email, err := ExtractEmailFromRequest(c.Request)
		if err == nil && email != "" {
			// Verify the email
			principal, err := authorizer.GetPrincipalFromEmail(c.Request.Context(), email, namespace)
			if err == nil && principal != nil {
				// Convert claims to principal and store in context
				c.Request = c.Request.WithContext(
					auth.WithPrincipal(c.Request.Context(), principal),
				)

				// Add principal to request logs
				SetRequestLogExtraFields(c.Request.Context(), map[string]any{
					"principal": principalRequestLogFields(principal),
				})
			}
		}

		// Continue to next handler (even if auth failed - let authorization middleware decide)
		c.Next()
	}
}

// RequireAuth is HTTP middleware that ensures a principal is present in the context.
// If no authenticated principal is found, it returns a 401 Unauthorized error.
//
// This should be used after AuthMiddleware in the middleware chain.
func RequireAuth[Identity ~string, Role ~string]() HandlerFunc {
	return func(c *Context) {
		principal := auth.GetPrincipal[Identity, Role](c.Request.Context())
		if principal == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{
				"error": "authentication required",
			})
			return
		}
		c.Next()
	}
}
