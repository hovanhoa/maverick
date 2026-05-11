package api

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextWithLoaders_roundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	loaders := &Loaders{}
	out := ContextWithLoaders(ctx, loaders)

	got, ok := out.Value(loadersCtxKey).(*Loaders)
	require.True(t, ok, "context should hold *Loaders under loadersCtxKey")
	assert.Same(t, loaders, got)
}

func TestCreateLoader_distinctPointersPerKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	loader := createLoader(
		func(ctx context.Context, keys []string) ([]string, error) {
			out := make([]string, 0, len(keys))
			for _, k := range keys {
				out = append(out, k+":payload")
			}
			return out, nil
		},
		func(s string) string {
			i := strings.Index(s, ":")
			if i < 0 {
				return s
			}
			return s[:i]
		},
	)

	a, err := loader.Load(ctx, "ka")
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, "ka:payload", *a)

	b, err := loader.Load(ctx, "kb")
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, "kb:payload", *b)
	assert.NotSame(t, a, b, "each key must get its own pointer, not a shared range variable address")
}

func TestCreateLoader_nilForMissingKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	loader := createLoader(
		func(ctx context.Context, keys []string) ([]string, error) {
			var out []string
			for _, k := range keys {
				if k == "present" {
					out = append(out, "present:ok")
				}
			}
			return out, nil
		},
		func(s string) string {
			return strings.SplitN(s, ":", 2)[0]
		},
	)

	v, err := loader.Load(ctx, "missing")
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestCreateLoader_LoadAll_singleFetch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var fetchCount int32
	loader := createLoader(
		func(ctx context.Context, keys []string) ([]string, error) {
			atomic.AddInt32(&fetchCount, 1)
			out := make([]string, 0, len(keys))
			for _, k := range keys {
				out = append(out, k+":x")
			}
			return out, nil
		},
		func(s string) string { return strings.SplitN(s, ":", 2)[0] },
	)

	vals, err := loader.LoadAll(ctx, []string{"a", "b", "c"})
	require.NoError(t, err)
	require.Len(t, vals, 3)
	for i, k := range []string{"a", "b", "c"} {
		require.NotNil(t, vals[i])
		assert.Equal(t, k+":x", *vals[i])
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&fetchCount), "LoadAll should batch keys into one fetch")
}

func TestNewLoaders_GetAccountByID_LoadAll(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	a1, err := database.CreateAccount(ctx, &model.Account{Email: "l1@example.com", Username: "load1"})
	require.NoError(t, err)
	a2, err := database.CreateAccount(ctx, &model.Account{Email: "l2@example.com", Username: "load2"})
	require.NoError(t, err)

	loaders := NewLoaders(database)
	got, err := loaders.GetAccountByID.LoadAll(ctx, []string{a1.ID, a2.ID, "account_absent"})
	require.NoError(t, err)
	require.Len(t, got, 3)

	byID := map[string]*model.Account{}
	for _, acc := range got[:2] {
		require.NotNil(t, acc)
		byID[acc.ID] = acc
	}
	assert.Equal(t, "l1@example.com", byID[a1.ID].Email)
	assert.Equal(t, "load2", byID[a2.ID].Username)
	assert.Nil(t, got[2])
}

func TestNewLoaders_GetAccountByID_Load(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	loaders := NewLoaders(database)
	require.NotNil(t, loaders.GetAccountByID)

	acc, err := database.CreateAccount(ctx, &model.Account{Email: "wire@example.com", Username: "wire"})
	require.NoError(t, err)
	out, err := loaders.GetAccountByID.Load(ctx, acc.ID)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, acc.ID, out.ID)
	assert.Equal(t, "wire@example.com", out.Email)
}
