package dto

import "time"

type RunRequest struct {
	RequestID string         `json:"request_id"`
	ToolName  string         `json:"tool_name"`
	ToolID    string         `json:"tool_id"`
	TimeoutMS int            `json:"timeout_ms"`
	Input     map[string]any `json:"input"`
	Context   map[string]any `json:"context"`
}

type RunResponse struct {
	RequestID   string          `json:"request_id"`
	Decision    string          `json:"decision"`
	ToolName    string          `json:"tool_name"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason,omitempty"`
	Result      any             `json:"result,omitempty"`
	LatencyMS   int64           `json:"latency_ms"`
	IntentID    string          `json:"intent_id,omitempty"`
	ApprovalID  string          `json:"approval_id,omitempty"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
}

type ExecuteIntentRequest struct {
	LeaseID string `json:"lease_id"`
}

type ErrorResponse struct {
	RequestID   string          `json:"request_id"`
	Error       ErrorObject     `json:"error"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type IdempotencyDTO struct {
	Present bool   `json:"present"`
	Outcome string `json:"outcome"`
}

type IntentItem struct {
	ID                   string         `json:"id"`
	RequestID            string         `json:"request_id"`
	ToolID               string         `json:"tool_id"`
	ToolName             string         `json:"tool_name"`
	PolicyID             *string        `json:"policy_id,omitempty"`
	RiskClass            string         `json:"risk_class"`
	Reason               string         `json:"reason"`
	Status               string         `json:"status"`
	PreflightStatus      string         `json:"preflight_status"`
	PreflightSummary     map[string]any `json:"preflight_summary"`
	PreflightCompletedAt *time.Time     `json:"preflight_completed_at,omitempty"`
	ApprovalID           *string        `json:"approval_id,omitempty"`
	ExpiresAt            time.Time      `json:"expires_at"`
	ExecutedAt           *time.Time     `json:"executed_at,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type ListIntentsResponse struct {
	Items []IntentItem `json:"items"`
}

type PreflightReviewResponse struct {
	IntentID     string         `json:"intent_id"`
	ToolName     string         `json:"tool_name"`
	RiskClass    string         `json:"risk_class"`
	Reason       string         `json:"reason"`
	Status       string         `json:"status"`
	Summary      map[string]any `json:"summary"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	ApprovalID   *string        `json:"approval_id,omitempty"`
	IntentStatus string         `json:"intent_status"`
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
