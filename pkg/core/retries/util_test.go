package retries_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/retries"
	"github.com/stretchr/testify/assert"
)

func TestGetBackoffWithJitter(t *testing.T) {
	origRandFloat64 := retries.RandFloat64
	t.Cleanup(func() {
		retries.RandFloat64 = origRandFloat64
	})

	tests := []struct {
		rand     float64
		backoff  time.Duration
		expected time.Duration
	}{
		{rand: 0.0, backoff: 1 * time.Second, expected: 750 * time.Millisecond},
		{rand: 0.5, backoff: 1 * time.Second, expected: 1000 * time.Millisecond},
		{rand: 1.0, backoff: 1 * time.Second, expected: 1250 * time.Millisecond},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.rand), func(t *testing.T) {
			retries.RandFloat64 = func() float64 { return test.rand }
			assert.Equal(t, test.expected, retries.GetBackoffWithJitter(test.backoff))
		})
	}
}
