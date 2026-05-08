package retries_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/retries"
	"github.com/stretchr/testify/assert"
)

func TestWithBackoff_Error(t *testing.T) {
	b := retries.Max(5).WithSleepFunc(func(d time.Duration) error { return nil })
	called := 0
	expectedErr := errors.New("test error")

	err := retries.WithBackoff(&b, func(r *retries.RetryControl) error {
		called++
		return expectedErr
	})

	assert.Equal(t, 6, called) // 1 try + 5 retries
	assert.Equal(t, expectedErr, err)
}

func TestWithBackoff_Success(t *testing.T) {
	b := retries.Max(5).WithSleepFunc(func(d time.Duration) error { return nil })
	called := 0

	err := retries.WithBackoff(&b, func(r *retries.RetryControl) error {
		called++
		if called == 3 {
			return nil
		}
		return errors.New("test error")
	})

	assert.Equal(t, 3, called) // 1 try + 2 retries
	assert.Nil(t, err)
}

func TestWithBackoff_StopRetries(t *testing.T) {
	b := retries.Max(5).WithSleepFunc(func(d time.Duration) error { return nil })
	called := 0
	expectedErr := errors.New("test error")

	err := retries.WithBackoff(&b, func(r *retries.RetryControl) error {
		called++
		if called == 3 {
			r.StopRetries()
			return expectedErr
		}
		return errors.New("not test error")
	})

	assert.Equal(t, 3, called) // 1 try + 2 retries
	assert.Equal(t, expectedErr, err)
}

func TestWithBackoff_SleepError(t *testing.T) {
	b := retries.Max(5).WithSleepFunc(func(d time.Duration) error { return errors.New("testing sleep error") })
	called := 0
	expectedErr := errors.New("testing sleep error")

	err := retries.WithBackoff(&b, func(r *retries.RetryControl) error {
		called++
		if called == 3 {
			r.StopRetries()
			return expectedErr
		}
		return errors.New("not test error")
	})

	assert.Equal(t, 1, called) // first try will fail due to sleep func error
	assert.Equal(t, expectedErr, err)
}

func TestRetryControl_HasMoreRetries(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		callCount  int
		expected   bool
	}{
		{
			name:       "has more retries at start",
			maxRetries: 5,
			callCount:  1,
			expected:   true,
		},
		{
			name:       "has more retries in middle",
			maxRetries: 5,
			callCount:  3,
			expected:   true,
		},
		{
			name:       "no more retries at max",
			maxRetries: 5,
			callCount:  6,
			expected:   false,
		},
		{
			name:       "no more retries when at limit",
			maxRetries: 3,
			callCount:  4,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := retries.Max(tt.maxRetries).WithSleepFunc(func(d time.Duration) error { return nil })
			called := 0
			var hasMore bool

			_ = retries.WithBackoff(&b, func(r *retries.RetryControl) error {
				called++
				hasMore = r.HasMoreRetries()
				if called >= tt.callCount {
					r.StopRetries()
					return nil
				}
				return errors.New("retry")
			})

			// At the point we stopped, check what HasMoreRetries returned on final call
			if tt.callCount > tt.maxRetries+1 {
				assert.False(t, hasMore)
			} else if tt.callCount == tt.maxRetries+1 {
				assert.False(t, hasMore)
			} else {
				assert.True(t, hasMore)
			}
		})
	}
}

func TestNewRetryControl(t *testing.T) {
	b := retries.Max(3)
	rc := retries.NewRetryControl(b)
	assert.NotNil(t, rc)
	assert.True(t, rc.HasMoreRetries())
}

func TestDefaultBackoff(t *testing.T) {
	b := retries.DefaultBackoff()
	assert.NotNil(t, b)
	assert.Equal(t, 10, b.MaxRetries)
	assert.Equal(t, 5*time.Second, b.Interval)
	assert.Equal(t, 1*time.Hour, b.MaxInterval)
	assert.Equal(t, 10*time.Second, b.MinInterval)
}

func TestBackoff_Reset(t *testing.T) {
	b := retries.Max(3).WithSleepFunc(func(d time.Duration) error { return nil })

	// Advance the backoff
	b.Next()
	b.Next()
	assert.Equal(t, 2, b.Attempt())

	// Reset should set attempt back to 0
	b.Reset()
	assert.Equal(t, 0, b.Attempt())
}

func TestBackoff_Increment(t *testing.T) {
	b := retries.Max(5)
	assert.Equal(t, 0, b.Attempt())

	b.Increment()
	assert.Equal(t, 1, b.Attempt())

	b.Increment()
	assert.Equal(t, 2, b.Attempt())
}

func TestBackoff_WithMaxJitter(t *testing.T) {
	b := retries.Max(3).WithMaxJitter(100 * time.Millisecond)
	assert.Equal(t, 100*time.Millisecond, b.MaxJitter)
}
