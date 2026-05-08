package retries_test

import (
	"math"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/retries"
	"github.com/stretchr/testify/assert"
)

type sleepAsserter struct {
	Exactly time.Duration
	Min     time.Duration
	Max     time.Duration
}

func (s sleepAsserter) Assert(t *testing.T, attempt int, b *retries.Backoff) {
	d := b.SleepDuration(attempt)
	if s.Exactly != 0 {
		assert.Equal(t, s.Exactly, d)
	} else {
		assert.LessOrEqual(t, d, s.Max)
		assert.GreaterOrEqual(t, d, s.Min)
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		Name     string
		Backoff  *retries.Backoff
		Expected []sleepAsserter
	}{
		{
			Name: "interval with max",
			Backoff: &retries.Backoff{
				Interval:    1 * time.Second,
				MaxInterval: 5 * time.Second,
			},
			Expected: []sleepAsserter{
				{Exactly: 1 * time.Second},
				{Exactly: 2 * time.Second},
				{Exactly: 4 * time.Second},
				{Exactly: 5 * time.Second},
				{Exactly: 5 * time.Second},
			},
		},
		{
			Name: "interval with min",
			Backoff: &retries.Backoff{
				Interval:    1 * time.Second,
				MinInterval: 3 * time.Second,
			},
			Expected: []sleepAsserter{
				{Exactly: 3 * time.Second},
				{Exactly: 3 * time.Second},
				{Exactly: 4 * time.Second},
				{Exactly: 8 * time.Second},
				{Exactly: 16 * time.Second},
			},
		},
		{
			Name: "interval with min and max",
			Backoff: &retries.Backoff{
				Interval:    1 * time.Second,
				MinInterval: 3 * time.Second,
				MaxInterval: 5 * time.Second,
			},
			Expected: []sleepAsserter{
				{Exactly: 3 * time.Second},
				{Exactly: 3 * time.Second},
				{Exactly: 4 * time.Second},
				{Exactly: 5 * time.Second},
				{Exactly: 5 * time.Second},
			},
		},
		{
			Name: "jitter",
			Backoff: &retries.Backoff{
				Interval:  1 * time.Second,
				MaxJitter: 100 * time.Millisecond,
			},
			Expected: []sleepAsserter{
				{Min: 900 * time.Millisecond, Max: 1100 * time.Millisecond},
				{Min: 1900 * time.Millisecond, Max: 2100 * time.Millisecond},
				{Min: 3900 * time.Millisecond, Max: 4100 * time.Millisecond},
			},
		},
		{
			Name: "jitter with min and max",
			Backoff: &retries.Backoff{
				Interval:    1 * time.Second,
				MaxJitter:   100 * time.Millisecond,
				MinInterval: 3 * time.Second,
				MaxInterval: 5 * time.Second,
			},
			Expected: []sleepAsserter{
				{Min: 2900 * time.Millisecond, Max: 3100 * time.Millisecond},
				{Min: 2900 * time.Millisecond, Max: 3100 * time.Millisecond},
				{Min: 3900 * time.Millisecond, Max: 4100 * time.Millisecond},
				{Min: 4900 * time.Millisecond, Max: 5100 * time.Millisecond},
				{Min: 4900 * time.Millisecond, Max: 5100 * time.Millisecond},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			for i, expected := range test.Expected {
				expected.Assert(t, i, test.Backoff)
			}
		})
	}
}

func TestBackoffSleep(t *testing.T) {
	last := 0
	b := &retries.Backoff{
		Interval:   1 * time.Second,
		MaxRetries: 5,
	}

	b.SleepFunc = func(d time.Duration) error {
		last++
		assert.Equal(t, last, b.Attempt())
		assert.Equal(t, b.Interval*time.Duration(math.Pow(2, float64(b.Attempt()))), d)
		return nil
	}

	i := 0
	for b.Next() {
		assert.Equal(t, i, last)
		err := b.Sleep()
		assert.NoError(t, err)
		assert.Equal(t, i+1, last)
		i++
	}
}

func TestNewBackoff(t *testing.T) {
	b0 := retries.Max(5)
	assert.Equal(t, 5, b0.MaxRetries)

	b1 := b0.WithInterval(200 * time.Millisecond)
	assert.Empty(t, b0.Interval)
	assert.Equal(t, 200*time.Millisecond, b1.Interval)

	b2 := b1.WithMaxInterval(200 * time.Millisecond)
	assert.Empty(t, b0.Interval)
	assert.Empty(t, b1.MaxInterval)
	assert.Equal(t, 200*time.Millisecond, b2.Interval)

	b3 := b2.WithMinInterval(200 * time.Millisecond)
	assert.Empty(t, b0.Interval)
	assert.Empty(t, b1.MaxInterval)
	assert.Empty(t, b2.MinInterval)
	assert.Equal(t, 200*time.Millisecond, b3.Interval)
	assert.Equal(t, 200*time.Millisecond, b3.MaxInterval)
	assert.Equal(t, 200*time.Millisecond, b3.MinInterval)
}
