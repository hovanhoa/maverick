package http

import (
	"context"

	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http"
)

type requestIDCtxKeyType string

const requestIDCtxKey requestIDCtxKeyType = "request_id"

// requestIDMiddleware assigns a request id (reusing an incoming X-Request-Id
// header if the client already set one), echoes it back on the response,
// and stores it in the request context for handlers/logging/usage
// metering to pick up.
func requestIDMiddleware(c *corehttp.Context) {
	id := c.Request.Header.Get("X-Request-Id")
	if id == "" {
		id = encoding.NewRandomIdentifier("req")
	}

	c.Writer.Header().Set("X-Request-Id", id)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDCtxKey, id))
	c.Next()
}

// requestIDFromContext returns the request id assigned by
// requestIDMiddleware, or "" if none is present (e.g. in a test that
// doesn't go through the middleware chain).
func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDCtxKey).(string)
	return id
}
