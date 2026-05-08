package http_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
)

func TestError_NewError(t *testing.T) {
	err := http.NewError(http.StatusBadRequest, "Test error %s", "format")
	assert.Equal(t, http.StatusBadRequest, err.StatusCode())
	assert.Equal(t, "Test error format", err.Error())
	assert.Regexp(t, regexp.MustCompile(`core/http/error_test.go:\d+\n`), fmt.Sprintf("%+v", err))
}

func TestError_NewInternalServerError(t *testing.T) {
	err := http.NewInternalServerError()
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode())
	assert.Equal(t, "An internal server error occurred", err.Error())
	assert.Regexp(t, regexp.MustCompile(`core/http/error_test.go:\d+\n`), fmt.Sprintf("%+v", err))
}

func TestFromError(t *testing.T) {
	t.Run("From HTTP error", func(t *testing.T) {
		httpError := http.NewError(http.StatusBadRequest, "http error").With(
			http.ErrorTypeOption(http.TypePreconditionFailed),
			http.ErrorCodeOption(http.CodeAlreadySubmitted),
			http.FieldOption("key", "value"),
		)
		err := http.FromError(httpError)
		assert.Equal(t, httpError, err)
		assert.Equal(t, http.StatusBadRequest, err.StatusCode())
		assert.Equal(t, http.TypePreconditionFailed, err.Type())
		assert.Equal(t, http.CodeAlreadySubmitted, err.Code())
		assert.Equal(t, "value", err.Field("key"))
	})

	t.Run("From validation error", func(t *testing.T) {
		err := http.FromError(http.NewValidationError("Field.SubField", "Message %s", "arg"))
		assert.Equal(t, http.StatusBadRequest, err.StatusCode())
		assert.Equal(t, http.TypeValidationError, err.Type())
		assert.Equal(t, http.CodeUnknown, err.Code())
		assert.Equal(t, "Message arg", err.Error())
		assert.Equal(t, "Field", err.Field("Field"))
		assert.Equal(t, "SubField", err.Field("SubField"))
	})

	t.Run("From an HTTP error in the middle of the stack", func(t *testing.T) {
		wrapErr := errors.Wrap(errors.New("inner"), "wrapper 1")
		httpErr := http.NewError(http.StatusBadRequest, "http error").With(
			http.CauseOption(wrapErr),
			http.ErrorTypeOption(http.TypePreconditionFailed),
			http.ErrorCodeOption(http.CodeAlreadySubmitted),
			http.FieldOption("key", "value"),
		)
		wrappedHttpErr := errors.Wrap(errors.Wrap(httpErr, "wrapper 2"), "wrapper 3")

		err := http.FromError(wrappedHttpErr)
		assert.Equal(t, wrappedHttpErr, err.Cause())
		assert.Equal(t, "http error: wrapper 3: wrapper 2: http error: wrapper 1: inner", err.Error())
		assert.Equal(t, http.StatusBadRequest, err.StatusCode())
		assert.Equal(t, http.TypePreconditionFailed, err.Type())
		assert.Equal(t, http.CodeAlreadySubmitted, err.Code())
		assert.Equal(t, "value", err.Field("key"))

		// Check that the stack trace is in the order we want
		trace := fmt.Sprintf("%+v", err)
		regex := regexp.MustCompile(`(?s)inner\n.+?wrapper 1\n.+?http error\n.+?wrapper 2\n.+?wrapper 3\n.+?http error\n`)
		assert.Regexp(t, regex, trace)
	})

	t.Run("From validation error in the middle of the stack", func(t *testing.T) {
		validationErr := http.NewValidationError("Field.SubField", "Message %s", "arg")
		wrappedHttpErr := errors.Wrap(errors.Wrap(validationErr, "wrapper 1"), "wrapper 2")

		err := http.FromError(wrappedHttpErr)
		assert.Nil(t, err.Cause())
		assert.Equal(t, "Message arg", err.Error())
		assert.Equal(t, http.StatusBadRequest, err.StatusCode())
		assert.Equal(t, http.TypeValidationError, err.Type())
		assert.Equal(t, http.CodeUnknown, err.Code())
		assert.Equal(t, "Field", err.Field("Field"))
		assert.Equal(t, "SubField", err.Field("SubField"))
	})

	t.Run("From a regular wrapped error", func(t *testing.T) {
		wrapErr := errors.Wrap(errors.New("inner"), "wrapper 1")
		err := http.FromError(wrapErr)
		assert.Equal(t, http.StatusInternalServerError, err.StatusCode())
		assert.Equal(t, "An internal server error occurred: wrapper 1: inner", err.Error())
		assert.Regexp(t, regexp.MustCompile(`core/http/error_test.go:\d+\n`), fmt.Sprintf("%+v", err))
	})
}

func TestWrapError(t *testing.T) {
	wrapErr := errors.New("inner")
	err := http.WrapError(wrapErr, "outer %s", "arg")
	assert.Equal(t, "An internal server error occurred: outer arg: inner", err.Error())
}

func TestError_MarshalJSON(t *testing.T) {
	httpError := http.NewError(http.StatusBadRequest, "http error").With(
		http.ErrorTypeOption(http.TypePreconditionFailed),
		http.ErrorCodeOption(http.CodeAlreadySubmitted),
		http.FieldOption("key", "value"),
	)

	data, err := json.Marshal(httpError)
	assert.NoError(t, err)

	parsedData := make(map[string]string)
	assert.NoError(t, json.Unmarshal(data, &parsedData))

	assert.Equal(t, map[string]string{
		"Message": "http error",
		"Type":    "precondition_failed",
		"Code":    "already_submitted",
		"key":     "value",
	}, parsedData)
}

func TestError_MarshalJSON_SkipEmpty(t *testing.T) {
	httpError := http.NewError(http.StatusBadRequest, "http error").With(
		http.FieldOption("key", ""),
		http.FieldOption("_key", "test"),
	)

	data, err := json.Marshal(httpError)
	assert.NoError(t, err)

	parsedData := make(map[string]string)
	assert.NoError(t, json.Unmarshal(data, &parsedData))

	assert.Equal(t, map[string]string{
		"Message": "http error",
	}, parsedData)
}

func TestNewValidationError(t *testing.T) {
	err := http.NewValidationError("Field", "Message %s", "arg")
	assert.Equal(t, "Field", err.Field)
	assert.Equal(t, "", err.SubField)
	assert.Equal(t, "Message arg", err.Message)
	assert.Equal(t, err.Message, err.Error())

	err = http.NewValidationError("Field.SubField", "Message %s", "arg")
	assert.Equal(t, "Field", err.Field)
	assert.Equal(t, "SubField", err.SubField)
	assert.Equal(t, "Message arg", err.Message)
	assert.Equal(t, err.Message, err.Error())

	err = http.NewValidationError("Field.SubField.SubSubField", "Message %s", "arg")
	assert.Equal(t, "Field", err.Field)
	assert.Equal(t, "SubField.SubSubField", err.SubField)
	assert.Equal(t, "Message arg", err.Message)
	assert.Equal(t, err.Message, err.Error())
}

func TestWrapValidationError(t *testing.T) {
	err := http.WrapValidationError(http.NewValidationError("Field", "Message %s", "arg"), "WrapField")
	assert.Equal(t, "WrapField", err.Field)
	assert.Equal(t, "Field", err.SubField)
	assert.Equal(t, "Message arg", err.Message)
	assert.Equal(t, err.Message, err.Error())
}

func TestWrapValidationError_NotValidationError(t *testing.T) {
	wrapErr := errors.New("generic error message")
	err := http.WrapValidationError(wrapErr, "WrapField")
	assert.Equal(t, "WrapField", err.Field)
	assert.Equal(t, "", err.SubField)
	assert.Equal(t, "generic error message", err.Message)
	assert.Equal(t, err.Message, err.Error())
}

func TestErrorFormat(t *testing.T) {
	err := http.NewError(http.StatusBadRequest, "Test error %s", "arg")
	assert.Equal(t, "Test error arg", fmt.Sprintf("%s", err))
	assert.Equal(t, "Test error arg", fmt.Sprintf("%v", err))
	assert.Equal(t, "\"Test error arg\"", fmt.Sprintf("%q", err))
}
