package graphql

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/hovanhoa/llmgateway/pkg/core/graphql/model"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func NewInternalServerError(ctx context.Context, message string, code model.InternalServerErrorCode, originalError error) *gqlerror.Error {
	return &gqlerror.Error{
		Err:     originalError,
		Path:    graphql.GetPath(ctx),
		Message: message,
		Extensions: map[string]interface{}{
			"internalServerError": &model.InternalServerError{
				Message: message,
			},
		},
	}
}

func NewApplicationError(ctx context.Context, message string, code model.ApplicationErrorCode, originalError error) *gqlerror.Error {
	return &gqlerror.Error{
		Err:     originalError,
		Path:    graphql.GetPath(ctx),
		Message: message,
		Extensions: map[string]interface{}{
			"applicationError": &model.ApplicationError{
				Code:    code,
				Message: message,
			},
		},
	}
}

func NewValidationError(ctx context.Context, message string, code model.ValidationErrorCode, fieldParts ...string) *gqlerror.Error {
	return &gqlerror.Error{
		Message: message,
		Path:    graphql.GetPath(ctx),
		Extensions: map[string]interface{}{
			"validationError": &model.ValidationError{
				Code:    code,
				Field:   fieldParts,
				Message: message,
			},
		},
	}
}

func isInternalServerError(err error) bool {
	gqlErr, ok := err.(*gqlerror.Error)
	return ok && gqlErr.Extensions != nil && gqlErr.Extensions["internalServerError"] != nil
}

func isApplicationError(err error) bool {
	gqlErr, ok := err.(*gqlerror.Error)
	return ok && gqlErr.Extensions != nil && gqlErr.Extensions["applicationError"] != nil
}

func isValidationError(err error) bool {
	gqlErr, ok := err.(*gqlerror.Error)
	return ok && gqlErr.Extensions != nil && gqlErr.Extensions["validationError"] != nil
}
