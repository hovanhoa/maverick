package proxy

import (
	"errors"
	"net/http"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

// ErrorResponseFor maps an error returned by Handler.ChatCompletion or
// Handler.StreamChatCompletion to an HTTP status code and an
// OpenAI-compatible error envelope.
func ErrorResponseFor(err error) (int, *openai.ErrorResponse) {
	var perr *provider.Error
	if !errors.As(err, &perr) {
		return http.StatusInternalServerError, openai.NewErrorResponse(openai.ErrorTypeInternal, err.Error())
	}

	switch perr.Kind {
	case provider.ErrorKindInvalidRequest:
		return http.StatusBadRequest, openai.NewErrorResponse(openai.ErrorTypeInvalidRequest, perr.Message)
	case provider.ErrorKindAuth:
		return http.StatusUnauthorized, openai.NewErrorResponse(openai.ErrorTypeAuthentication, perr.Message)
	case provider.ErrorKindQuota:
		return http.StatusTooManyRequests, openai.NewErrorResponse(openai.ErrorTypeRateLimit, perr.Message)
	case provider.ErrorKindPolicy:
		return http.StatusForbidden, openai.NewErrorResponse(openai.ErrorTypePermission, perr.Message)
	case provider.ErrorKindTimeout:
		return http.StatusGatewayTimeout, openai.NewErrorResponse(openai.ErrorTypeUpstream, perr.Message)
	case provider.ErrorKindTransient:
		return http.StatusBadGateway, openai.NewErrorResponse(openai.ErrorTypeUpstream, perr.Message)
	default:
		return http.StatusInternalServerError, openai.NewErrorResponse(openai.ErrorTypeInternal, perr.Message)
	}
}
