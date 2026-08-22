package proxy

import (
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	quotaDeniedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "llmgateway",
		Subsystem: "quota",
		Name:      "denied_total",
		Help:      "Total number of proxy requests denied for exceeding a team's monthly token budget.",
	}, []string{"team_id"})

	// model is deliberately not a label here: it comes from the caller's
	// raw request (proxy.resolve just splits "provider/model" - it never
	// validates modelName against a known list, and a team with no
	// allowlist configured accepts any model string), so it isn't a safe,
	// bounded Prometheus label. provider is safe - it's bounded by the
	// fixed provider registry.
	streamDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "llmgateway",
		Subsystem: "proxy",
		Name:      "stream_duration_seconds",
		Help:      "Duration of a streaming chat completion call, from the upstream connection opening to it closing.",
		Buckets:   apm.RequestDurationSecondsHistogramBuckets,
	}, []string{"provider", "status"})
)

func init() {
	apm.DefineMetric(quotaDeniedTotal, streamDurationSeconds)
}
