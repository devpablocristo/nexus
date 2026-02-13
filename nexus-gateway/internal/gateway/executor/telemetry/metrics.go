package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type RunMetrics struct {
	runCounter metric.Int64Counter
	runLatency metric.Int64Histogram
}

func NewRunMetrics() *RunMetrics {
	meter := otel.Meter("nexus-gateway/run")
	runCounter, _ := meter.Int64Counter("nexus_run_total")
	runLatency, _ := meter.Int64Histogram("nexus_run_latency_ms")
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
}
