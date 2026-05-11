package api

import (
	"context"

	dbpkg "github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/vikstrous/dataloadgen"
)

type loadersCtxKeyType string

const loadersCtxKey loadersCtxKeyType = "net.hovanhoa.llmgateway.internal.api.loaders"

type Loaders struct {
	// Single value loaders
	GetAccountByID *dataloadgen.Loader[string, *model.Account]
}

func NewLoaders(db *dbpkg.Database) *Loaders {
	return &Loaders{
		// Single value loaders
		GetAccountByID: createLoader(
			db.GetAccountsByIDs,
			func(account model.Account) string { return account.ID },
		),
	}
}

func ContextWithLoaders(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, loadersCtxKey, loaders)
}

func createLoader[K comparable, V any](
	loaderFunc func(ctx context.Context, vals []K) ([]V, error),
	keyFunc func(val V) K,
) *dataloadgen.Loader[K, *V] {
	return dataloadgen.NewMappedLoader(func(ctx context.Context, keys []K) (map[K]*V, error) {
		vals, err := loaderFunc(ctx, keys)
		if err != nil {
			return nil, err
		}

		mapVals := make(map[K]*V)
		for i := range vals {
			mapVals[keyFunc(vals[i])] = &vals[i]
		}

		for _, key := range keys {
			if _, ok := mapVals[key]; !ok {
				mapVals[key] = nil
			}
		}

		return mapVals, nil
	})
}
