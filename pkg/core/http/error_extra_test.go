package http_test

import (
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
)

func TestNewUnauthorizedError(t *testing.T) {
	err := http.NewUnauthorizedError()
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode())
	assert.Equal(t, "You must be logged in to access this endpoint", err.Error())
}

func TestNewErrorWithTypeAndCode(t *testing.T) {
	err := http.NewErrorWithTypeAndCode(
		http.StatusForbidden,
		http.TypePreconditionFailed,
		http.CodeAlreadySubmitted,
		"forbidden %s", "resource",
	)
	assert.Equal(t, http.StatusForbidden, err.StatusCode())
	assert.Equal(t, http.TypePreconditionFailed, err.Type())
	assert.Equal(t, http.CodeAlreadySubmitted, err.Code())
	assert.Equal(t, "forbidden resource", err.Error())
}

func TestError_Field(t *testing.T) {
	t.Run("no_extra_fields", func(t *testing.T) {
		err := http.NewError(http.StatusBadRequest, "test")
		assert.Equal(t, "", err.Field("anything"))
	})

	t.Run("with_extra_fields", func(t *testing.T) {
		err := http.NewError(http.StatusBadRequest, "test").With(
			http.FieldOption("key", "value"),
		)
		assert.Equal(t, "value", err.Field("key"))
		assert.Equal(t, "", err.Field("missing"))
	})
}

func TestError_GetValidationError(t *testing.T) {
	t.Run("is_validation_error", func(t *testing.T) {
		err := http.FromError(http.NewValidationError("Field.Sub", "msg"))
		ve := err.GetValidationError()
		assert.NotNil(t, ve)
		assert.Equal(t, "Field", ve.Field)
		assert.Equal(t, "Sub", ve.SubField)
		assert.Equal(t, "msg", ve.Message)
	})

	t.Run("not_validation_error", func(t *testing.T) {
		err := http.NewError(http.StatusBadRequest, "test")
		ve := err.GetValidationError()
		assert.Nil(t, ve)
	})
}

func TestCombineMaps(t *testing.T) {
	// combineMaps is unexported, but we can test it via FieldOption + MarshalJSON
	err := http.NewError(http.StatusBadRequest, "test").With(
		http.FieldOption("key1", "val1"),
		http.FieldOption("key2", "val2"),
	)
	assert.Equal(t, "val1", err.Field("key1"))
	assert.Equal(t, "val2", err.Field("key2"))
}
