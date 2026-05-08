package http

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// Error is an HTTP error, specifically to be returned at the service boundary to
// an HTTP handler. Its purpose is to separate out the error that is communicated
// to the user from the internal cause of the error, though sometimes these may be
// the same.
type Error struct {
	msg   string
	cause error
	stack *errors.Stack

	errorCode   ErrorCode
	errorType   ErrorType
	statusCode  int
	extraFields map[string]string
}

// NewError creates a new HTTP error with the given status code and message.
// It records the stack track at the point this function is called.
func NewError(statusCode int, message string, args ...interface{}) *Error {
	return &Error{
		msg:        fmt.Sprintf(message, args...),
		stack:      errors.Callers(),
		statusCode: statusCode,
	}
}

func NewErrorWithTypeAndCode(statusCode int, errorType ErrorType, errorCode ErrorCode, message string, args ...interface{}) *Error {
	return &Error{
		msg:        fmt.Sprintf(message, args...),
		stack:      errors.Callers(),
		statusCode: statusCode,
		errorType:  errorType,
		errorCode:  errorCode,
	}
}

// NewInternalServerError creates a new HTTP internal server error with a
// fixed message and status code 500.
func NewInternalServerError() *Error {
	return &Error{
		msg:        "An internal server error occurred",
		stack:      errors.Callers(),
		statusCode: StatusInternalServerError,
	}
}

// NewInternalServerError creates a new HTTP unauthorized error with a
// fixed message and status code 401.
func NewUnauthorizedError() *Error {
	return &Error{
		msg:        "You must be logged in to access this endpoint",
		stack:      errors.Callers(),
		statusCode: StatusUnauthorized,
	}
}

// FromError converts a generic error into an HTTP error by first examining
// some special cases and otherwise returning an internal server error with
// the given error as the cause. In order, this function:
//
//  1. Checks to see if the error is already an HTTP error, and if so, returns
//     it directly.
//  2. Iteratively unwraps the error until it finds an HTTP error, and if so,
//     returns a new HTTP error containing the given error as the cause and
//     the found HTTP error's HTTP properties (message, code, type, fields, etc.)
//  3. Iteratively unwraps the error until it finds a validation error, and if
//     so, returns a new HTTP error with a nil cause, setting its error type to
//     TypeValidationError, status to StatusBadRequest, and settings its message
//     and extra fields based on the validation error.
//  4. Returns an internal server error with the given error as the cause.
func FromError(err error) *Error {
	// if err is an http error, return it
	if httpError, ok := err.(*Error); ok {
		return httpError
	}

	// iterate over the error chain to search for specific error
	// types and handle them appropriately
	iter := err
	for iter != nil {
		// if err is a wrapped error that has an http error, create
		// an http error with that spec and wrap err with it
		if httpError, ok := iter.(*Error); ok {
			return &Error{
				cause:       err,
				stack:       errors.Callers(),
				msg:         httpError.msg,
				errorCode:   httpError.errorCode,
				errorType:   httpError.errorType,
				statusCode:  httpError.statusCode,
				extraFields: httpError.extraFields,
			}
		}

		// if err is a validation error, create an http error using the
		// validation error fields
		if validationError, ok := iter.(*ValidationError); ok {
			return &Error{
				stack:      errors.Callers(),
				msg:        validationError.Message,
				errorType:  TypeValidationError,
				statusCode: StatusBadRequest,
				extraFields: map[string]string{
					"Field":    validationError.Field,
					"SubField": validationError.SubField,
				},
			}
		}

		cause, ok := iter.(interface{ Cause() error })
		if !ok {
			break
		}

		iter = cause.Cause()
	}

	// else, return an internal server error with err as the cause
	return NewInternalServerError().With(
		CauseOption(err),
		StackOption(errors.Callers()),
	)
}

// WrapError provides convenient access to errors.Wrap for HTTP errors
// by effectively calling http.FromError(errors.Wrap(...)) and setting
// properties like the status code and stack trace. Like other Error
// functions, this function is meant to be called as the final wrap in
// an HTTP error chain.
func WrapError(err error, message string, args ...interface{}) *Error {
	return FromError(errors.Wrap(err, message, args...)).With(StackOption(errors.Callers()))
}

// With applies a sequence of error options to override properties
// of the Error. It returns a new Error with the modifications applied.
// See error_option.go for all potential options.
func (e Error) With(opts ...ErrorOption) *Error {
	for _, opt := range opts {
		e = opt(e)
	}
	return &e
}

// Cause returns the cause of this error, and implements the Causer interface.
func (e *Error) Cause() error {
	return e.cause
}

// Error returns a string representation of this error, including all wrapped
// errors, and implements the Error interface.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s", e.msg, e.cause.Error())
	}
	return e.msg
}

// Format implements the Formatter interface and augments the
// default method of stringifying an Error with a modifier
// that causes the stack track of this error and all of its
// causes to be written (via the %+v directive).
func (e *Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			if e.Cause() != nil {
				_, _ = fmt.Fprintf(s, "%+v\n", e.Cause())
			}
			_, _ = fmt.Fprintf(s, "%s", e.msg)
			e.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}

// MarshalJSON implements the json.Marshaler interface and
// provides a method for serializing this error to JSON for
// consumption by clients. It adds the Type and Code fields,
// mapping to e.errorType and e.errorCode respectively, as well
// as all `e.extraFields` verbatim, skipping those that have an
// empty value or start with `_.“
func (e *Error) MarshalJSON() ([]byte, error) {
	m := map[string]string{
		"Type":    string(e.errorType),
		"Code":    string(e.errorCode),
		"Message": e.Error(),
	}
	for k, v := range e.extraFields {
		m[k] = v
	}
	for k, v := range m {
		if k[0] == '_' || v == "" {
			delete(m, k)
		}
	}
	return json.Marshal(m)
}

// StatusCode returns the HTTP status code associated with this error.
func (e *Error) StatusCode() int {
	return e.statusCode
}

// Code returns the error code associated with this error.
func (e *Error) Code() ErrorCode {
	return e.errorCode
}

// Type returns the error type associated with this error.
func (e *Error) Type() ErrorType {
	return e.errorType
}

// Field returns the value for the extra field with the given key
// associated with this error. It returns an empty string if no
// value is found.
func (e *Error) Field(key string) string {
	if e.extraFields != nil {
		return e.extraFields[key]
	}

	return ""
}

func (e *Error) GetValidationError() *ValidationError {
	if e.Type() == TypeValidationError {
		return &ValidationError{
			Field:    e.Field("Field"),
			SubField: e.Field("SubField"),
			Message:  e.msg,
		}
	}
	return nil
}

func combineMaps(maps ...map[string]string) map[string]string {
	res := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			if k != "" && v != "" {
				res[k] = v
			}
		}
	}
	return res
}

// ValidationError represents a bottom-of-the-stack error in which
// validation failed. It allows the user to return a custom message
// and point the caller to the specific field that the error is for.
type ValidationError struct {
	Field    string
	SubField string
	Message  string
}

// Error implements the Error interface and returns the error message
// associated with the validation error.
func (v ValidationError) Error() string {
	return v.Message
}

// NewValidationError returns a new ValidationError for the given field with
// the given message.
func NewValidationError(field string, msg string, args ...interface{}) *ValidationError {
	fieldParts := strings.SplitN(strings.Trim(field, "."), ".", 2)
	field = fieldParts[0]

	var subField string
	if len(fieldParts) > 1 {
		subField = fieldParts[1]
	}

	return &ValidationError{Field: field, SubField: subField, Message: fmt.Sprintf(msg, args...)}
}

// WrapValidationError checks if the given error is a validation error, and if so,
// returns a new but identical error except it uses the field prefix as the Field
// property, and transfers the existing Field property to the SubField property.
func WrapValidationError(err error, fieldPrefix string) *ValidationError {
	validationErr, ok := errors.Cause(err).(*ValidationError)
	if !ok {
		return NewValidationError(fieldPrefix, "%s", err.Error())
	}

	return NewValidationError(
		strings.Join(
			[]string{
				fieldPrefix,
				validationErr.Field,
				validationErr.SubField,
			},
			".",
		),
		"%s",
		validationErr.Message,
	)
}
