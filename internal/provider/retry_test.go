package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRetry_SucceedsFirstTry(t *testing.T) {
	t.Parallel()

	calls := 0
	result, err := provider.WithRetry(context.Background(), 3, func() (string, error) {
		calls++
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, 1, calls)
}

func TestWithRetry_RetriesTransientUntilSuccess(t *testing.T) {
	t.Parallel()

	calls := 0
	result, err := provider.WithRetry(context.Background(), 3, func() (string, error) {
		calls++
		if calls < 3 {
			return "", provider.NewError("test", provider.ErrorKindTransient, "temporary", nil)
		}
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, 3, calls)
}

func TestWithRetry_ExhaustsRetries(t *testing.T) {
	t.Parallel()

	calls := 0
	_, err := provider.WithRetry(context.Background(), 2, func() (string, error) {
		calls++
		return "", provider.NewError("test", provider.ErrorKindTransient, "always fails", nil)
	})

	require.Error(t, err)
	assert.Equal(t, 3, calls, "1 initial attempt + 2 retries")
}

func TestWithRetry_NonTransientErrorNotRetried(t *testing.T) {
	t.Parallel()

	calls := 0
	_, err := provider.WithRetry(context.Background(), 3, func() (string, error) {
		calls++
		return "", provider.NewError("test", provider.ErrorKindAuth, "invalid key", nil)
	})

	require.Error(t, err)
	assert.Equal(t, 1, calls, "auth errors must not be retried")
}

func TestWithRetry_NonProviderErrorNotRetried(t *testing.T) {
	t.Parallel()

	calls := 0
	_, err := provider.WithRetry(context.Background(), 3, func() (string, error) {
		calls++
		return "", errors.New("boom")
	})

	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestWithRetry_ContextCancelledStopsRetrying(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	_, err := provider.WithRetry(ctx, 5, func() (string, error) {
		calls++
		return "", provider.NewError("test", provider.ErrorKindTransient, "temporary", nil)
	})

	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestWithRetry_RespectsTimingBudget(t *testing.T) {
	t.Parallel()

	start := time.Now()
	_, _ = provider.WithRetry(context.Background(), 2, func() (string, error) {
		return "", provider.NewError("test", provider.ErrorKindTransient, "temporary", nil)
	})
	// Backoff intervals here are tens/hundreds of ms; the whole test should
	// still finish well under a second.
	assert.Less(t, time.Since(start), 2*time.Second)
}
