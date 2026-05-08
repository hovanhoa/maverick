package auth

import (
	"context"

	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
)

type principalCtxKeyType string

const principalCtxKey principalCtxKeyType = "net.oysterinc.gocode.pkg.core.auth.principal"

// WithPrincipal adds a principal to the context and configures APM and logging.
// This should be called by authentication middleware after verifying credentials.
func WithPrincipal[Identity ~string, Role ~string](ctx context.Context, principal *Principal[Identity, Role]) context.Context {
	if principal == nil {
		return ctx
	}

	// Configure APM to track the user
	if hub := apm.GetHubFromContext(ctx); hub != nil {
		hub.ConfigureScope(func(scope *apm.Scope) {
			var user apm.User
			user.ID = principal.ID
			user.Email = principal.Email
			user.Name = principal.Name
			scope.SetUser(user)
		})
	}

	// Add principal to logger context
	ctx = log.ContextWithLogger(ctx, log.FromContext(ctx).With(log.Any("principal", map[string]interface{}{
		"id":       principal.ID,
		"type":     principal.Type,
		"email":    principal.Email,
		"name":     principal.Name,
		"org_id":   principal.OrgID,
		"org_name": principal.OrgName,
	})))

	return context.WithValue(ctx, principalCtxKey, principal)
}

// GetPrincipal retrieves the principal from the context.
// Returns nil if no principal is present in the context.
func GetPrincipal[Identity ~string, Role ~string](ctx context.Context) *Principal[Identity, Role] {
	if val := ctx.Value(principalCtxKey); val != nil {
		if principal, ok := val.(*Principal[Identity, Role]); ok {
			return principal
		}
	}
	return nil
}
