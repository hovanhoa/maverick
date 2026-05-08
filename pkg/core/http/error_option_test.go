package http_test

import (
	"fmt"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
)

func TestHTTPErrorOption(t *testing.T) {
	err := http.NewError(http.StatusInternalServerError, "test %s", "error")
	assert.Equal(t, "test error", err.Error())
	assert.Equal(t, http.TypeUnknown, err.Type())
	assert.Equal(t, http.CodeUnknown, err.Code())
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode())

	nextErr := err.With(http.ErrorCodeOption(http.CodeUnsupportedGeo))
	assert.Equal(t, http.CodeUnknown, err.Code())
	assert.Equal(t, http.CodeUnsupportedGeo, nextErr.Code())

	err = nextErr
	nextErr = err.With(http.ErrorTypeOption(http.TypeUnderwritingError))
	assert.Equal(t, http.TypeUnknown, err.Type())
	assert.Equal(t, http.TypeUnderwritingError, nextErr.Type())

	err = nextErr
	nextErr = err.With(http.StatusCodeOption(http.StatusBadRequest))
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode())
	assert.Equal(t, http.StatusBadRequest, nextErr.StatusCode())

	err = nextErr
	cause := errors.New("cause")
	nextErr = err.With(http.CauseOption(cause))
	assert.Nil(t, err.Cause())
	assert.Equal(t, cause, nextErr.Cause())

	err = nextErr
	nextErr = err.With(http.FieldOption("key", "value"))
	assert.Equal(t, "", err.Field("key"))
	assert.Equal(t, "value", nextErr.Field("key"))

	err = nextErr
	nextErr = err.With(http.StackOption(errors.Callers()))
	assert.Contains(t, fmt.Sprintf("%+v", err), "error_option_test.go:13")
	assert.Contains(t, fmt.Sprintf("%+v", err), "error_option_test.go:34")
	assert.NotContains(t, fmt.Sprintf("%+v", nextErr), "error_option_test.go:13")
	assert.Contains(t, fmt.Sprintf("%+v", nextErr), "error_option_test.go:34")

	// Check that the final error contains all the new option values
	assert.Equal(t, http.CodeUnsupportedGeo, nextErr.Code())
	assert.Equal(t, http.TypeUnderwritingError, nextErr.Type())
	assert.Equal(t, http.StatusBadRequest, nextErr.StatusCode())
	assert.Equal(t, cause, nextErr.Cause())
	assert.Equal(t, "value", nextErr.Field("key"))
	assert.NotContains(t, fmt.Sprintf("%+v", nextErr), "error_option_test.go:13")
	assert.Contains(t, fmt.Sprintf("%+v", nextErr), "error_option_test.go:34")
}
