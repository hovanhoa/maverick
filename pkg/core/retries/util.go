package retries

import (
	"context"
	"math/rand"
	"time"

	"github.com/benbjohnson/clock"
)

var (
	RandFloat64 = rand.Float64
)

// Algorithm uses `backoff` as the median and chooses a random value
// in the interval [0.75N, 1.25N]
func GetBackoffWithJitter(backoff time.Duration) time.Duration {
	// [0, 1] -> [0, 0.5] -> [0.75, 1.25]
	coeff := 0.5*RandFloat64() + 0.75
	return time.Duration(coeff * float64(backoff))
}

func Sleep(ctx context.Context, c clock.Clock, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.After(duration):
		return nil
	}
}
