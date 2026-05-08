package apm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionID_NilContext(t *testing.T) {
	t.Parallel()

	// When there's no Sentry transaction in the context, should return empty string
	ctx := context.Background()
	result := GetTransactionID(ctx)
	assert.Empty(t, result, "GetTransactionID() should return empty string")
}

func TestGetTransactionID_EmptyContext(t *testing.T) {
	t.Parallel()

	// Use a fresh context with no transaction
	ctx := context.TODO()
	result := GetTransactionID(ctx)
	assert.Empty(t, result, "GetTransactionID() should return empty string")
}

func TestGetRegistry(t *testing.T) {
	t.Parallel()

	registry := GetRegistry()
	require.NotNil(t, registry, "GetRegistry() returned nil")

	// Verify it's a valid Prometheus registry by checking we can gather metrics
	_, err := registry.Gather()
	assert.NoError(t, err, "Registry.Gather() should not error")
}

func TestGetRegistry_HasGoCollector(t *testing.T) {
	t.Parallel()

	registry := GetRegistry()
	metrics, err := registry.Gather()
	require.NoError(t, err, "Registry.Gather() should not error")

	// Check that Go collector metrics are present
	hasGoMetrics := false
	for _, m := range metrics {
		if m.GetName() == "go_goroutines" || m.GetName() == "go_threads" {
			hasGoMetrics = true
			break
		}
	}

	assert.True(t, hasGoMetrics, "Expected Go collector metrics to be registered")
}

func TestDefineMetric(t *testing.T) {
	// Note: Not running in parallel since it modifies global registry

	// Use unique metric name to avoid conflicts when running with -count flag
	metricName := fmt.Sprintf("test_apm_counter_%d", time.Now().UnixNano())

	// Create a test counter
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: metricName,
		Help: "A test counter for APM tests",
	})

	// Register it
	DefineMetric(counter)

	// Verify it's in the registry
	registry := GetRegistry()
	metrics, err := registry.Gather()
	require.NoError(t, err, "Registry.Gather() should not error")

	found := false
	for _, m := range metrics {
		if m.GetName() == metricName {
			found = true
			break
		}
	}

	assert.True(t, found, "DefineMetric() did not register the counter in the registry")
}

func TestRequestDurationSecondsHistogramBuckets(t *testing.T) {
	t.Parallel()

	// Verify the bucket values are defined and reasonable
	require.NotEmpty(t, RequestDurationSecondsHistogramBuckets, "RequestDurationSecondsHistogramBuckets is empty")

	// Check buckets are in ascending order
	for i := 1; i < len(RequestDurationSecondsHistogramBuckets); i++ {
		assert.Greater(t, RequestDurationSecondsHistogramBuckets[i], RequestDurationSecondsHistogramBuckets[i-1],
			"Buckets not in ascending order at index %d: %f <= %f",
			i, RequestDurationSecondsHistogramBuckets[i], RequestDurationSecondsHistogramBuckets[i-1])
	}

	// First bucket should be small (< 1 second)
	assert.Less(t, RequestDurationSecondsHistogramBuckets[0], 1.0, "First bucket should be < 1.0")

	// Last bucket should be reasonably large
	last := RequestDurationSecondsHistogramBuckets[len(RequestDurationSecondsHistogramBuckets)-1]
	assert.GreaterOrEqual(t, last, 10.0, "Last bucket should be >= 10.0")
}

func TestRequestSizeBytesHistogramBuckets(t *testing.T) {
	t.Parallel()

	// Verify the bucket values are defined
	require.NotEmpty(t, RequestSizeBytesHistogramBuckets, "RequestSizeBytesHistogramBuckets is empty")

	// Should have same number of buckets as duration buckets
	assert.Len(t, RequestSizeBytesHistogramBuckets, len(RequestDurationSecondsHistogramBuckets),
		"RequestSizeBytesHistogramBuckets should have same length as RequestDurationSecondsHistogramBuckets")

	// Buckets should be 100x the duration buckets
	for i, v := range RequestSizeBytesHistogramBuckets {
		expected := RequestDurationSecondsHistogramBuckets[i] * 100
		assert.Equal(t, expected, v, "RequestSizeBytesHistogramBuckets[%d] mismatch", i)
	}
}
