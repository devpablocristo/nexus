package dto

import "time"

type RunRequest struct {
	RequestID string         `json:"request_id"`
	ToolName  string         `json:"tool_name"`
	ToolID    string         `json:"tool_id"`
	Input     map[string]any `json:"input" binding:"required"`
	Context   map[string]any `json:"context"`
}

type ExecuteIntentRequest struct {
	LeaseID string `json:"lease_id" binding:"required,uuid"`
}

type RunSuccessResponse struct {
	RequestID   string          `json:"request_id"`
	Decision    string          `json:"decision"`
	ToolName    string          `json:"tool_name"`
	Status      string          `json:"status"`
	Result      any             `json:"result"`
	LatencyMS   int64           `json:"latency_ms"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
	IntentID    *string         `json:"intent_id,omitempty"`
	ApprovalID  *string         `json:"approval_id,omitempty"`
	RiskClass   *string         `json:"risk_class,omitempty"`
	LeaseID     *string         `json:"lease_id,omitempty"`
}

type RunBlockedResponse struct {
	RequestID   string          `json:"request_id"`
	Decision    string          `json:"decision"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason"`
	Error       any             `json:"error"`
	LatencyMS   int64           `json:"latency_ms"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
	IntentID    *string         `json:"intent_id,omitempty"`
	ApprovalID  *string         `json:"approval_id,omitempty"`
	RiskClass   *string         `json:"risk_class,omitempty"`
	LeaseID     *string         `json:"lease_id,omitempty"`
}

type RunErrorResponse struct {
	RequestID   string          `json:"request_id"`
	Decision    string          `json:"decision"`
	Status      string          `json:"status"`
	Error       any             `json:"error"`
	LatencyMS   int64           `json:"latency_ms"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
	IntentID    *string         `json:"intent_id,omitempty"`
	ApprovalID  *string         `json:"approval_id,omitempty"`
	RiskClass   *string         `json:"risk_class,omitempty"`
	LeaseID     *string         `json:"lease_id,omitempty"`
}

type SimulateResponse struct {
	RequestID string         `json:"request_id"`
	Decision  string         `json:"decision"`
	ToolName  string         `json:"tool_name"`
	Status    string         `json:"status"`
	Reason    string         `json:"reason,omitempty"`
	Error     any            `json:"error,omitempty"`
	Explain   map[string]any `json:"explain"`
	LatencyMS int64          `json:"latency_ms"`
}

type IdempotencyDTO struct {
	Present bool   `json:"present"`
	Outcome string `json:"outcome"`
}

type IntentItem struct {
	ID                   string         `json:"id"`
	RequestID            string         `json:"request_id"`
	ToolName             string         `json:"tool_name"`
	Actor                *string        `json:"actor,omitempty"`
	Role                 *string        `json:"role,omitempty"`
	Scopes               []string       `json:"scopes"`
	Input                map[string]any `json:"input"`
	Context              map[string]any `json:"context"`
	PolicyID             *string        `json:"policy_id,omitempty"`
	RiskClass            string         `json:"risk_class"`
	Reason               string         `json:"reason"`
	ApprovalID           *string        `json:"approval_id,omitempty"`
	Status               string         `json:"status"`
	PreflightStatus      string         `json:"preflight_status"`
	PreflightSummary     map[string]any `json:"preflight_summary"`
	PreflightArtifactSHA *string        `json:"preflight_artifact_sha256,omitempty"`
	PreflightCompletedAt *time.Time     `json:"preflight_completed_at,omitempty"`
	ExpiresAt            time.Time      `json:"expires_at"`
	ApprovedAt           *time.Time     `json:"approved_at,omitempty"`
	ExecutedAt           *time.Time     `json:"executed_at,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type ListIntentsResponse struct {
	Items []IntentItem `json:"items"`
}

type PreflightReviewResponse struct {
	IntentID       string         `json:"intent_id"`
	ToolName       string         `json:"tool_name"`
	RiskClass      string         `json:"risk_class"`
	Reason         string         `json:"reason"`
	Status         string         `json:"status"`
	Summary        map[string]any `json:"summary"`
	ArtifactSHA256 *string        `json:"artifact_sha256,omitempty"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	ApprovalID     *string        `json:"approval_id,omitempty"`
	IntentStatus   string         `json:"intent_status"`
}

type ExecutionLeaseItem struct {
	ID              string         `json:"id"`
	IntentID        string         `json:"intent_id"`
	ToolName        string         `json:"tool_name"`
	RiskClass       string         `json:"risk_class"`
	Status          string         `json:"status"`
	CredentialMode  string         `json:"credential_mode"`
	CredentialHints map[string]any `json:"credential_hints"`
	ExpiresAt       time.Time      `json:"expires_at"`
	UsedAt          *time.Time     `json:"used_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}
