package telemetry

import (
	"context"
	"sync"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type RunMetrics struct {
	runCounter metric.Int64Counter
	runLatency metric.Int64Histogram
}

var (
	promOnce         sync.Once
	promRunTotal     *prom.CounterVec
	promRunLatencyMS *prom.HistogramVec
)

func NewRunMetrics() *RunMetrics {
	meter := otel.Meter("nexus-core/run")
	runCounter, _ := meter.Int64Counter("nexus_run_total")
	runLatency, _ := meter.Int64Histogram("nexus_run_latency_ms")
	promOnce.Do(func() {
		promRunTotal = prom.NewCounterVec(prom.CounterOpts{
			Name: "nexus_run_total_prom",
			Help: "Total run outcomes by tool/decision/status",
		}, []string{"tool_name", "decision", "status"})
		promRunLatencyMS = prom.NewHistogramVec(prom.HistogramOpts{
			Name:    "nexus_run_latency_ms_prom",
			Help:    "Run latency in milliseconds by tool/decision/status",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		}, []string{"tool_name", "decision", "status"})
		prom.MustRegister(promRunTotal, promRunLatencyMS)
	})
	return &RunMetrics{
		runCounter: runCounter,
		runLatency: runLatency,
	}
}

func (m *RunMetrics) ObserveRun(ctx context.Context, toolName, decision, status string, latency time.Duration) {
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
		attribute.String("decision", decision),
		attribute.String("status", status),
	}
	m.runCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.runLatency.Record(ctx, latency.Milliseconds(), metric.WithAttributes(attrs...))
	if promRunTotal != nil && promRunLatencyMS != nil {
		promRunTotal.WithLabelValues(toolName, decision, status).Inc()
		promRunLatencyMS.WithLabelValues(toolName, decision, status).Observe(float64(latency.Milliseconds()))
	}
}
