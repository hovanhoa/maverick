package graphql

import (
	"context"
	"fmt"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/graphql/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestNewInternalServerError(t *testing.T) {
	t.Parallel()

	originalErr := fmt.Errorf("database connection failed")
	err := NewInternalServerError(context.Background(), "internal error", model.InternalServerErrorCodeUnknown, originalErr)

	require.NotNil(t, err)
	assert.Equal(t, "internal error", err.Message)
	assert.Equal(t, originalErr, err.Err)
	assert.NotNil(t, err.Extensions)
	assert.NotNil(t, err.Extensions["internalServerError"])

	ise, ok := err.Extensions["internalServerError"].(*model.InternalServerError)
	require.True(t, ok)
	assert.Equal(t, "internal error", ise.Message)
}

func TestNewApplicationError(t *testing.T) {
	t.Parallel()

	originalErr := fmt.Errorf("not found")
	err := NewApplicationError(context.Background(), "resource not found", model.ApplicationErrorCodeNotFound, originalErr)

	require.NotNil(t, err)
	assert.Equal(t, "resource not found", err.Message)
	assert.Equal(t, originalErr, err.Err)

	ae, ok := err.Extensions["applicationError"].(*model.ApplicationError)
	require.True(t, ok)
	assert.Equal(t, model.ApplicationErrorCodeNotFound, ae.Code)
	assert.Equal(t, "resource not found", ae.Message)
}

func TestNewValidationError(t *testing.T) {
	t.Parallel()

	err := NewValidationError(context.Background(), "field required", model.ValidationErrorCodeRequired, "input", "email")

	require.NotNil(t, err)
	assert.Equal(t, "field required", err.Message)
	assert.Nil(t, err.Err)

	ve, ok := err.Extensions["validationError"].(*model.ValidationError)
	require.True(t, ok)
	assert.Equal(t, model.ValidationErrorCodeRequired, ve.Code)
	assert.Equal(t, "field required", ve.Message)
	assert.Equal(t, []string{"input", "email"}, ve.Field)
}

func TestIsInternalServerError(t *testing.T) {
	t.Parallel()

	t.Run("true for internal server error", func(t *testing.T) {
		err := NewInternalServerError(context.Background(), "oops", model.InternalServerErrorCodeUnknown, nil)
		assert.True(t, isInternalServerError(err))
	})

	t.Run("false for application error", func(t *testing.T) {
		err := NewApplicationError(context.Background(), "not found", model.ApplicationErrorCodeNotFound, nil)
		assert.False(t, isInternalServerError(err))
	})

	t.Run("false for plain error", func(t *testing.T) {
		assert.False(t, isInternalServerError(fmt.Errorf("plain error")))
	})

	t.Run("false for gqlerror without extensions", func(t *testing.T) {
		err := &gqlerror.Error{Message: "no extensions"}
		assert.False(t, isInternalServerError(err))
	})
}

func TestIsApplicationError(t *testing.T) {
	t.Parallel()

	t.Run("true for application error", func(t *testing.T) {
		err := NewApplicationError(context.Background(), "not found", model.ApplicationErrorCodeNotFound, nil)
		assert.True(t, isApplicationError(err))
	})

	t.Run("false for internal server error", func(t *testing.T) {
		err := NewInternalServerError(context.Background(), "oops", model.InternalServerErrorCodeUnknown, nil)
		assert.False(t, isApplicationError(err))
	})

	t.Run("false for plain error", func(t *testing.T) {
		assert.False(t, isApplicationError(fmt.Errorf("plain error")))
	})
}

func TestIsValidationError(t *testing.T) {
	t.Parallel()

	t.Run("true for validation error", func(t *testing.T) {
		err := NewValidationError(context.Background(), "required", model.ValidationErrorCodeRequired)
		assert.True(t, isValidationError(err))
	})

	t.Run("false for application error", func(t *testing.T) {
		err := NewApplicationError(context.Background(), "not found", model.ApplicationErrorCodeNotFound, nil)
		assert.False(t, isValidationError(err))
	})

	t.Run("false for plain error", func(t *testing.T) {
		assert.False(t, isValidationError(fmt.Errorf("plain error")))
	})
}
