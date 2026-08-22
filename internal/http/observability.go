package http

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http"
)

type requestIDCtxKeyType string

const requestIDCtxKey requestIDCtxKeyType = "request_id"

// observabilityMiddleware assigns a request id (reusing an incoming
// X-Request-Id header if the client already set one), echoes it back on
// the response, and stores it in the request context for
// handlers/logging/usage metering to pick up. It also attaches request id,
// account id, and (if any) team id to the canonical request log line via
// RequestExtraFieldsContextKey - the "request id, team, account" fields
// the Phase 5 plan calls for, alongside the latency/status the canonical
// line already logs for every request.
//
// Must run after auth, since the account/team fields depend on the
// principal auth resolved.
func observabilityMiddleware(c *corehttp.Context) {
	id := c.Request.Header.Get("X-Request-Id")
	if id == "" {
		id = encoding.NewRandomIdentifier("req")
	}

	c.Writer.Header().Set("X-Request-Id", id)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDCtxKey, id))

	fields := map[string]any{"requestId": id}
	if principal := auth.GetPrincipal[model.Identity, model.Role](c.Request.Context()); principal != nil {
		fields["accountId"] = principal.ID
		if principal.OrgID != "" {
			fields["teamId"] = principal.OrgID
		}
	}
	c.Set(string(corehttp.RequestExtraFieldsContextKey), fields)

	c.Next()
}

// requestIDFromContext returns the request id assigned by
// observabilityMiddleware, or "" if none is present (e.g. in a test that
// doesn't go through the middleware chain).
func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDCtxKey).(string)
	return id
}
