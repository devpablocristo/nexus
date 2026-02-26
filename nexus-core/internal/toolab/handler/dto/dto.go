package dto

import domain "nexus-core/internal/toolab/usecases/domain"

// ManifestResponse is returned by GET /_toolab/manifest.
type ManifestResponse = domain.Manifest

// FingerprintResponse is returned by GET /_toolab/state/fingerprint.
type FingerprintResponse struct {
	Fingerprint string `json:"fingerprint"`
	Scope       string `json:"scope"`
	Timestamp   string `json:"timestamp"`
}

// SnapshotRequest is the body for POST /_toolab/state/snapshot.
type SnapshotRequest struct {
	Label string `json:"label"`
}

// SnapshotResponse is returned by POST /_toolab/state/snapshot.
type SnapshotResponse struct {
	SnapshotID  string `json:"snapshot_id"`
	Fingerprint string `json:"fingerprint"`
	Label       string `json:"label"`
	CreatedAt   string `json:"created_at"`
}

// RestoreRequest is the body for POST /_toolab/state/restore.
type RestoreRequest struct {
	SnapshotID string `json:"snapshot_id"`
}

// RestoreResponse is returned by POST /_toolab/state/restore.
type RestoreResponse struct {
	Restored    bool   `json:"restored"`
	SnapshotID  string `json:"snapshot_id"`
	Fingerprint string `json:"fingerprint"`
}

// ResetResponse is returned by POST /_toolab/state/reset.
type ResetResponse struct {
	Reset       bool   `json:"reset"`
	Fingerprint string `json:"fingerprint"`
}

// MetricResponse is a single metric entry in GET /_toolab/metrics.
type MetricResponse struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Value  any               `json:"value"`
	Labels map[string]string `json:"labels"`
}

// MetricsResponse is returned by GET /_toolab/metrics.
type MetricsResponse struct {
	CollectedAt string           `json:"collected_at"`
	Metrics     []MetricResponse `json:"metrics"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SchemaResponse is returned by GET /_toolab/schema.
type SchemaResponse = domain.SchemaResponse

// SuggestedFlowsResponse is returned by GET /_toolab/suggested_flows.
type SuggestedFlowsResponse = domain.SuggestedFlowsResponse

// InvariantsResponse is returned by GET /_toolab/invariants.
type InvariantsResponse = domain.InvariantsResponse

// LimitsResponse is returned by GET /_toolab/limits.
type LimitsResponse = domain.LimitsResponse

// EnvironmentResponse is returned by GET /_toolab/environment.
type EnvironmentResponse = domain.EnvironmentResponse

// ProfileResponse is returned by GET /_toolab/profile.
type ProfileResponse = domain.ProfileResponse
