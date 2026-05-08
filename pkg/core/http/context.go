package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Context is a framework-specific type used to interact with the HTTP
// context a request handler is operating within.
type Context = gin.Context

// NewTestContext creates a new context to use in tests.
func NewTestContext(w http.ResponseWriter, r *http.Request) *Context {
	g, _ := gin.CreateTestContext(w)
	g.Request = r
	return g
}

type (
	requestErrorContextKeyType             string
	requestErrorSetterContextKeyType       string
	requestExtraFieldsContextKeyType       string
	requestExtraFieldsSetterContextKeyType string
)

const (
	RequestErrorContextKey             requestErrorContextKeyType             = "request_error"
	RequestErrorSetterContextKey       requestErrorSetterContextKeyType       = "request_error_setter"
	RequestExtraFieldsContextKey       requestExtraFieldsContextKeyType       = "request_extra_fields"
	RequestExtraFieldsSetterContextKey requestExtraFieldsSetterContextKeyType = "request_extra_fields_setter"
)

type RequestErrorSetter func(err error)
type RequestExtraFieldsSetter func(fields map[string]interface{})

func ContextWithRequestHeaders(ctx context.Context, req *http.Request) context.Context {
	// Store request headers in the context so that the HTTP service client can propagate them
	// to downstream services.
	authorization := req.Header.Get(AuthorizationHeaderKey)
	if authorization != "" {
		ctx = context.WithValue(ctx, AuthorizationContextKey, authorization)
	}
	cookie := req.Header.Get(GetSessionCookieID())
	if cookie != "" {
		ctx = context.WithValue(ctx, SessionCookieContextKey, cookie)
	}

	return ctx
}

func SetRequestLogExtraFields(ctx context.Context, fields map[string]interface{}) {
	if setterValue := ctx.Value(RequestExtraFieldsSetterContextKey); setterValue != nil {
		if setter, ok := setterValue.(RequestExtraFieldsSetter); ok {
			setter(fields)
		}
	}
}

func SetRequestLogError(ctx context.Context, err error) {
	if setterValue := ctx.Value(RequestErrorSetterContextKey); setterValue != nil {
		if setter, ok := setterValue.(RequestErrorSetter); ok {
			setter(err)
		}
	}
}
