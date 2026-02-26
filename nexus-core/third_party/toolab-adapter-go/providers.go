package toolab

import (
	"context"
	"time"
)

// StateProvider manages application data state for reproducibility.
type StateProvider interface {
	// Fingerprint returns a deterministic hash of the current data state.
	// Same data MUST produce the same fingerprint.
	// Format: "algo:hex" (e.g., "sha256:abc123...")
	Fingerprint(ctx context.Context) (string, error)

	// Snapshot captures the current state and returns an ID and fingerprint.
	Snapshot(ctx context.Context, label string) (id string, fingerprint string, err error)

	// Restore restores state to a previous snapshot.
	Restore(ctx context.Context, snapshotID string) error

	// Reset restores to initial/seed state.
	Reset(ctx context.Context) error
}

// MetricsProvider collects structured metrics snapshots.
type MetricsProvider interface {
	// Snapshot returns the current metrics.
	Snapshot(ctx context.Context) ([]Metric, error)
}

// Metric is a single metric data point.
type Metric struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"` // "counter", "gauge", "histogram"
	Value  any               `json:"value"`
	Labels map[string]string `json:"labels"`
}

// HistogramValue is the value type for histogram metrics.
type HistogramValue struct {
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Count int64   `json:"count"`
	Sum   float64 `json:"sum"`
}

// LogsProvider collects structured log lines.
type LogsProvider interface {
	// Collect returns log lines since the given time, up to limit.
	// If level is non-empty, only lines at that level or above are returned.
	Collect(ctx context.Context, since time.Time, limit int, level string) ([]LogLine, error)
}

// LogLine is a single structured log entry.
type LogLine struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Attrs     map[string]any `json:"attrs,omitempty"`
}

// TracesProvider collects trace spans.
type TracesProvider interface {
	// Collect returns traces since the given time, up to limit.
	Collect(ctx context.Context, since time.Time, limit int) ([]Trace, error)
}

// Trace is a single trace span.
type Trace struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Operation  string            `json:"operation"`
	DurationMS int               `json:"duration_ms"`
	Status     string            `json:"status"`
	StartedAt  string            `json:"started_at"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// SeedProvider handles deterministic seed propagation.
type SeedProvider interface {
	// Apply sets the application into deterministic mode with the given seed.
	// scope lists what to make deterministic (e.g., "uuid", "timestamp", "jitter").
	Apply(ctx context.Context, seed string, scope []string) (SeedResult, error)

	// Clear exits deterministic mode.
	Clear(ctx context.Context) error
}

// SeedResult reports what was actually made deterministic.
type SeedResult struct {
	Applied []string `json:"applied"`
	Ignored []string `json:"ignored"`
}

// SchemaProvider exposes schema capability data.
type SchemaProvider interface {
	Schema(ctx context.Context) (any, error)
}

// SuggestedFlowsProvider exposes suggested_flows capability data.
type SuggestedFlowsProvider interface {
	SuggestedFlows(ctx context.Context) (any, error)
}

// InvariantsProvider exposes invariants capability data.
type InvariantsProvider interface {
	Invariants(ctx context.Context) (any, error)
}

// LimitsProvider exposes limits capability data.
type LimitsProvider interface {
	Limits(ctx context.Context) (any, error)
}

// EnvironmentProvider exposes environment capability data.
type EnvironmentProvider interface {
	Environment(ctx context.Context) (any, error)
}

// OpenAPIInfo is metadata advertised for the openapi capability.
type OpenAPIInfo struct {
	URL         string `json:"url,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Version     string `json:"version,omitempty"`
	ETag        string `json:"etag,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

// OpenAPIProvider exposes OpenAPI document and metadata for capability fallback.
type OpenAPIProvider interface {
	OpenAPIDocument(ctx context.Context) (contentType string, document []byte, err error)
	OpenAPIInfo(ctx context.Context) (*OpenAPIInfo, error)
}
