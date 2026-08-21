package provider

import "fmt"

// ErrorKind is a standardized taxonomy for provider call failures, used by
// the proxy layer to map errors to HTTP status/OpenAI-compatible error
// types and to decide whether a retry is worthwhile.
type ErrorKind string

const (
	// ErrorKindInvalidRequest means the provider rejected the request as
	// malformed (bad model name, unsupported parameter, etc).
	ErrorKindInvalidRequest ErrorKind = "invalid_request"
	// ErrorKindAuth means the gateway's credentials for the provider were
	// rejected.
	ErrorKindAuth ErrorKind = "auth"
	// ErrorKindQuota means the provider rate-limited or quota-limited the
	// request.
	ErrorKindQuota ErrorKind = "quota"
	// ErrorKindTimeout means the call did not complete before its deadline.
	ErrorKindTimeout ErrorKind = "timeout"
	// ErrorKindTransient means a retry is likely to succeed (connection
	// reset, 5xx, etc).
	ErrorKindTransient ErrorKind = "transient"
	// ErrorKindPolicy means the provider refused the request on content
	// policy grounds.
	ErrorKindPolicy ErrorKind = "policy"
	// ErrorKindUnknown is used when a failure can't be classified into any
	// of the above.
	ErrorKindUnknown ErrorKind = "unknown"
)

// Error wraps a provider call failure with its ErrorKind. It implements
// Unwrap so errors.As/errors.Is work against the underlying cause.
type Error struct {
	Provider string
	Kind     ErrorKind
	Message  string
	Cause    error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %s: %v", e.Provider, e.Kind, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s: %s", e.Provider, e.Kind, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

// NewError builds a provider Error.
func NewError(providerName string, kind ErrorKind, message string, cause error) *Error {
	return &Error{Provider: providerName, Kind: kind, Message: message, Cause: cause}
}
