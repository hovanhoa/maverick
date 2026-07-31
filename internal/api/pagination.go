package api

import (
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

const (
	// DefaultPageLimit is the page size used when a query omits limit.
	DefaultPageLimit = 20

	// MaxPageLimit is the largest page size a query may request. Larger values
	// are clamped rather than rejected.
	MaxPageLimit = 100
)

// resolvePage normalizes the optional limit and offset arguments of a paginated
// query into concrete bounds. Nonsensical values are rejected; an oversized
// limit is clamped to MaxPageLimit.
func resolvePage(limit *int, offset *int) (int, int, error) {
	resolvedLimit := DefaultPageLimit
	if limit != nil {
		if *limit < 1 {
			return 0, 0, errors.New("limit must be greater than 0")
		}
		resolvedLimit = min(*limit, MaxPageLimit)
	}

	resolvedOffset := 0
	if offset != nil {
		if *offset < 0 {
			return 0, 0, errors.New("offset must not be negative")
		}
		resolvedOffset = *offset
	}

	return resolvedLimit, resolvedOffset, nil
}

// hasNextPage reports whether more rows follow the page that was just read.
func hasNextPage(offset int, pageSize int, totalCount int) bool {
	return offset+pageSize < totalCount
}
