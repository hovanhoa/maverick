package openai

// ErrorType matches the values OpenAI's API uses in its error envelope, so
// clients written against OpenAI's SDKs handle gateway errors unmodified.
type ErrorType string

const (
	ErrorTypeInvalidRequest ErrorType = "invalid_request_error"
	ErrorTypeAuthentication ErrorType = "authentication_error"
	ErrorTypeRateLimit      ErrorType = "rate_limit_error"
	ErrorTypePermission     ErrorType = "permission_error"
	ErrorTypeUpstream       ErrorType = "upstream_error"
	ErrorTypeInternal       ErrorType = "internal_error"
)

// ErrorDetail is the body of an ErrorResponse.
type ErrorDetail struct {
	Message string    `json:"message"`
	Type    ErrorType `json:"type"`
	Param   string    `json:"param,omitempty"`
	Code    string    `json:"code,omitempty"`
}

// ErrorResponse is the canonical, OpenAI-compatible error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// NewErrorResponse builds an ErrorResponse with the given type and message.
func NewErrorResponse(errType ErrorType, message string) *ErrorResponse {
	return &ErrorResponse{Error: ErrorDetail{Message: message, Type: errType}}
}
