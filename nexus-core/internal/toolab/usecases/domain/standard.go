package domain

// Manifest describes TOOLAB adapter capabilities and discovery links.
type Manifest struct {
	AdapterVersion  string            `json:"adapter_version"`
	StandardVersion string            `json:"standard_version"`
	AppName         string            `json:"app_name"`
	AppVersion      string            `json:"app_version"`
	Capabilities    []string          `json:"capabilities"`
	Links           map[string]string `json:"links,omitempty"`
}

// SchemaResponse is returned by GET /_toolab/schema.
type SchemaResponse struct {
	Database         DatabaseInfo      `json:"database"`
	Entities         []EntityInfo      `json:"entities"`
	OperationEffects []OperationEffect `json:"operation_effects,omitempty"`
}

// DatabaseInfo contains storage engine metadata.
type DatabaseInfo struct {
	Type       string `json:"type"`
	Version    string `json:"version,omitempty"`
	SchemaName string `json:"schema_name,omitempty"`
}

// EntityInfo describes a single entity/table.
type EntityInfo struct {
	Name                  string       `json:"name"`
	Table                 string       `json:"table"`
	Description           string       `json:"description,omitempty"`
	Columns               []ColumnInfo `json:"columns"`
	EstimatedRowCountHint *int64       `json:"estimated_row_count_hint,omitempty"`
}

// ColumnInfo describes a table column.
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	PK       bool   `json:"pk,omitempty"`
	FK       string `json:"fk,omitempty"`
}

// OperationEffect optionally maps API operations to DB effects.
type OperationEffect struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Effect     string `json:"effect"`
	Entity     string `json:"entity,omitempty"`
	Idempotent bool   `json:"idempotent"`
}

// SuggestedFlowsResponse is returned by GET /_toolab/suggested_flows.
type SuggestedFlowsResponse struct {
	Flows          []SuggestedFlow   `json:"flows"`
	DefaultHeaders map[string]string `json:"default_headers,omitempty"`
}

// SuggestedFlow is a reusable flow definition.
type SuggestedFlow struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description,omitempty"`
	Weight      int                    `json:"weight,omitempty"`
	Requests    []SuggestedFlowRequest `json:"requests"`
}

// SuggestedFlowRequest is one request inside a flow.
type SuggestedFlowRequest struct {
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          map[string]string `json:"query,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Body           *string           `json:"body,omitempty"`
	JSONBody       map[string]any    `json:"json_body,omitempty"`
	TimeoutMS      int               `json:"timeout_ms,omitempty"`
	Weight         int               `json:"weight,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
}

// InvariantsResponse is returned by GET /_toolab/invariants.
type InvariantsResponse struct {
	Invariants []Invariant `json:"invariants"`
}

// Invariant defines one invariant advertised by the service.
type Invariant struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Description string         `json:"description,omitempty"`
	Max         *float64       `json:"max,omitempty"`
	Status      *int           `json:"status,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

// LimitsResponse is returned by GET /_toolab/limits.
type LimitsResponse struct {
	Rate        *RateLimits        `json:"rate,omitempty"`
	Quotas      []QuotaLimit       `json:"quotas,omitempty"`
	Timeouts    *TimeoutLimits     `json:"timeouts,omitempty"`
	Concurrency *ConcurrencyLimits `json:"concurrency,omitempty"`
	Payload     *PayloadLimits     `json:"payload,omitempty"`
}

type RateLimits struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	Burst             int     `json:"burst,omitempty"`
	WindowSeconds     int     `json:"window_seconds,omitempty"`
}

type QuotaLimit struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Period string  `json:"period,omitempty"`
}

type TimeoutLimits struct {
	RequestDefaultMS int `json:"request_default_ms,omitempty"`
	RequestMaxMS     int `json:"request_max_ms,omitempty"`
}

type ConcurrencyLimits struct {
	MaxInflight int `json:"max_inflight,omitempty"`
	MaxQueue    int `json:"max_queue,omitempty"`
}

type PayloadLimits struct {
	MaxRequestBodyBytes  int64 `json:"max_request_body_bytes,omitempty"`
	MaxResponseBodyBytes int64 `json:"max_response_body_bytes,omitempty"`
	MaxLogsLines         int   `json:"max_logs_lines,omitempty"`
	MaxTracesSpans       int   `json:"max_traces_spans,omitempty"`
}

// EnvironmentResponse is returned by GET /_toolab/environment.
type EnvironmentResponse struct {
	Mode     string          `json:"mode"`
	ReadOnly bool            `json:"read_only"`
	Features map[string]bool `json:"features"`
	Metadata map[string]any  `json:"metadata,omitempty"`
	Release  *ReleaseInfo    `json:"release,omitempty"`
}

type ReleaseInfo struct {
	Version    string `json:"version,omitempty"`
	GitSHA     string `json:"git_sha,omitempty"`
	DeployedAt string `json:"deployed_at,omitempty"`
}

// OpenAPIInfo contains OpenAPI discovery metadata.
type OpenAPIInfo struct {
	URL         string `json:"url,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Version     string `json:"version,omitempty"`
	ETag        string `json:"etag,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

// ProfileResponse aggregates discovery payloads.
type ProfileResponse struct {
	StandardVersion string                  `json:"standard_version"`
	ProfileVersion  string                  `json:"profile_version"`
	Manifest        *Manifest               `json:"manifest,omitempty"`
	Schema          *SchemaResponse         `json:"schema,omitempty"`
	SuggestedFlows  *SuggestedFlowsResponse `json:"suggested_flows,omitempty"`
	Invariants      *InvariantsResponse     `json:"invariants,omitempty"`
	Limits          *LimitsResponse         `json:"limits,omitempty"`
	Environment     *EnvironmentResponse    `json:"environment,omitempty"`
	OpenAPI         *OpenAPIInfo            `json:"openapi,omitempty"`
	Unknowns        []string                `json:"unknowns,omitempty"`
	Hashes          map[string]string       `json:"hashes,omitempty"`
}
