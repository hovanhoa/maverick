package apm

import (
	"github.com/hovanhoa/llmgateway/pkg/core/collections"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var reg *prometheus.Registry

var (
	RequestDurationSecondsHistogramBuckets = []float64{0.01, 0.05, 0.10, 0.25, 0.50, 0.75, 1.00, 1.50, 2.00, 2.50, 5.00, 7.50, 10.00, 15.00, 20.00, 25.00, 30.00}
	RequestSizeBytesHistogramBuckets       = collections.Map(RequestDurationSecondsHistogramBuckets, func(v float64) float64 { return v * 100 })
)

func GetRegistry() *prometheus.Registry {
	return reg
}

func DefineMetric(metrics ...prometheus.Collector) {
	reg.MustRegister(metrics...)
}

func init() {
	reg = prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}
