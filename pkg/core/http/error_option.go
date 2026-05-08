package http

import "github.com/hovanhoa/llmgateway/pkg/core/errors"

// ErrorOption modifies an error object in a specific way. Use any of the *Option methods
// below to customize the error.
type ErrorOption func(Error) Error

// ErrorCodeOption attaches the specified error code to the error.
func ErrorCodeOption(code ErrorCode) ErrorOption {
	return func(h Error) Error {
		h.errorCode = code
		return h
	}
}

// ErrorTypeOption attaches the specified error type to the error.
func ErrorTypeOption(t ErrorType) ErrorOption {
	return func(h Error) Error {
		h.errorType = t
		return h
	}
}

// StatusCodeOption attaches the specified HTTP status code to the error.
func StatusCodeOption(code int) ErrorOption {
	return func(h Error) Error {
		h.statusCode = code
		return h
	}
}

// CauseOption attaches the specified error as the cause to the HTTP error.
func CauseOption(err error) ErrorOption {
	return func(h Error) Error {
		h.cause = err
		return h
	}
}

// StackOption attaches the specified stack trace to the error.
func StackOption(stack *errors.Stack) ErrorOption {
	return func(h Error) Error {
		h.stack = stack
		return h
	}
}

// FieldOption attaches the specified key-value pair to the error.
func FieldOption(key, value string) ErrorOption {
	return func(h Error) Error {
		h.extraFields = combineMaps(h.extraFields, map[string]string{key: value})
		return h
	}
}
