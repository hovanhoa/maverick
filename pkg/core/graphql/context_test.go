package graphql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryContext(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for empty context", func(t *testing.T) {
		ctx := context.Background()
		qc := GetQueryContext(ctx)
		assert.Nil(t, qc)
	})

	t.Run("round-trips through WithQueryContext", func(t *testing.T) {
		ctx := context.Background()
		qc := &QueryContext{}
		ctx = WithQueryContext(ctx, qc)
		got := GetQueryContext(ctx)
		require.NotNil(t, got)
		assert.Equal(t, qc, got)
	})

	t.Run("returns nil for wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), gqlRequestContext, "not a QueryContext")
		qc := GetQueryContext(ctx)
		assert.Nil(t, qc)
	})
}
