package graphql

import (
	"context"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
)

type gqlContextType string

const (
	gqlRequestContext = gqlContextType("OysterGraphQLQueryRequestContext")
)

type QueryContext struct {
	Request        *http.Request
	ResponseWriter http.ResponseWriter
}

func setQueryContextFromHTTP() http.HandlerFunc {
	return func(c *http.Context) {
		c.Request = c.Request.WithContext(
			WithQueryContext(c.Request.Context(), &QueryContext{
				Request:        c.Request,
				ResponseWriter: c.Writer,
			}),
		)
	}
}

func GetQueryContext(ctx context.Context) *QueryContext {
	if val := ctx.Value(gqlRequestContext); val != nil {
		if queryContext, ok := val.(*QueryContext); ok {
			return queryContext
		}
	}

	return nil
}

func WithQueryContext(ctx context.Context, queryContext *QueryContext) context.Context {
	return context.WithValue(ctx, gqlRequestContext, queryContext)
}
