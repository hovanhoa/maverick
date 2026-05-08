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

func getLoaders(ctx context.Context) *Loaders {
	if val := ctx.Value(loadersCtxKey); val != nil {
		if loaders, ok := val.(*Loaders); ok {
			return loaders
		}
	}
	return nil
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
		for _, val := range vals {
			mapVals[keyFunc(val)] = &val
		}

		for _, key := range keys {
			if _, ok := mapVals[key]; !ok {
				mapVals[key] = nil
			}
		}

		return mapVals, nil
	})
}

// createManyLoader creates a multi-value loader that batches requests by key.
// The loaderFunc MUST fetch all items for all keys in a SINGLE database query.
// The keyFunc extracts the foreign key from each result to group by.
//
// Example:
//
//	createManyLoader(
//	    func(ctx context.Context, agencyIDs []string) ([]Account, error) {
//	        // Single query: WHERE agency_id IN (...)
//	        return db.GetAccountsByAgencyIDs(ctx, agencyIDs)
//	    },
//	    func(account Account) string { return account.AgencyID },
//	)
func createManyLoader[K comparable, V any](
	loaderFunc func(ctx context.Context, keys []K) ([]V, error),
	keyFunc func(val V) K,
) *dataloadgen.Loader[K, []V] {
	return dataloadgen.NewMappedLoader(func(ctx context.Context, keys []K) (map[K][]V, error) {
		if len(keys) == 0 {
			return make(map[K][]V), nil
		}

		// Fetch all values in a single batched query
		allVals, err := loaderFunc(ctx, keys)
		if err != nil {
			return nil, err
		}

		// Group values by their foreign key
		mapVals := make(map[K][]V, len(keys))
		for _, val := range allVals {
			key := keyFunc(val)
			mapVals[key] = append(mapVals[key], val)
		}

		// Ensure all requested keys have entries (empty slice if no results)
		for _, key := range keys {
			if _, ok := mapVals[key]; !ok {
				mapVals[key] = []V{}
			}
		}

		return mapVals, nil
	})
}

func getOptionalKey[K comparable](key *K) K {
	if key == nil {
		var zeroVal K
		return zeroVal
	}
	return *key
}

func convertFromSliceOfPointers[V any](vals []*V, err error) ([]V, error) {
	if err != nil {
		return nil, err
	}
	var nonNilVals []V
	for _, val := range vals {
		if val != nil {
			nonNilVals = append(nonNilVals, *val)
		}
	}
	return nonNilVals, nil
}
