package domain

import "time"

// SnapshotMeta holds metadata about a captured state snapshot.
type SnapshotMeta struct {
	ID          string
	Fingerprint string
	Label       string
	CreatedAt   time.Time
}

// MetricItem represents a single structured metric from the application.
type MetricItem struct {
	Name   string
	Type   string // "counter", "gauge", "histogram", "summary", "untyped"
	Value  any
	Labels map[string]string
}
