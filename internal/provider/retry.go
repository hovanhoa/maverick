package provider

import (
	"context"
	"errors"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/retries"
)

// WithRetry calls fn, retrying while it returns an *Error whose Kind is
// ErrorKindTransient, up to maxRetries additional attempts with exponential
// backoff (so maxRetries=2 means at most 3 total calls to fn). Any other
// error - including context cancellation - is returned immediately.
func WithRetry[T any](ctx context.Context, maxRetries int, fn func() (T, error)) (T, error) {
	backoff := retries.NewBackoff().
		WithInterval(100 * time.Millisecond).
		WithMinInterval(50 * time.Millisecond).
		WithMaxInterval(2 * time.Second)

	for attempt := 0; ; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		var perr *Error
		if !errors.As(err, &perr) || perr.Kind != ErrorKindTransient {
			return result, err
		}
		if attempt >= maxRetries {
			return result, err
		}
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(backoff.SleepDuration(attempt)):
		}
	}
}
