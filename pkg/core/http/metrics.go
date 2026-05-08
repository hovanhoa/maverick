package http

import (
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "core",
		Subsystem: "http",
		Name:      "request_count",
		Help:      "The total number of requests in a given period of time",
	}, []string{"method", "path", "status"})
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "core",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "The duration of the request in milliseconds",
		Buckets:   apm.RequestDurationSecondsHistogramBuckets,
	}, []string{"method", "path", "status"})
	requestSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "core",
		Subsystem: "http",
		Name:      "request_size_bytes",
		Help:      "The size of the request body in bytes",
		Buckets:   apm.RequestSizeBytesHistogramBuckets,
	}, []string{"method", "path", "status"})
	responseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "core",
		Subsystem: "http",
		Name:      "response_size_bytes",
		Help:      "The size of the response body in bytes",
		Buckets:   apm.RequestSizeBytesHistogramBuckets,
	}, []string{"method", "path", "status"})
	httpClientRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "core",
		Subsystem: "http_client",
		Name:      "request_total",
		Help:      "Total number of outbound HTTP requests",
	}, []string{"external_service", "method", "status", "path"})

	httpClientRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "core",
		Subsystem: "http_client",
		Name:      "request_duration_seconds",
		Help:      "Duration of outbound HTTP requests",
		Buckets:   apm.RequestDurationSecondsHistogramBuckets,
	}, []string{"external_service", "method", "status", "path"})
)

func init() {
	apm.DefineMetric(
		requestCount,
		requestDuration,
		requestSize,
		responseSize,
		httpClientRequestTotal,
		httpClientRequestDuration,
	)
}
