package dto

import "time"

type AuditItem struct {
	RequestID  string    `json:"request_id"`
	OrgID      string    `json:"org_id,omitempty"`
	ToolName   string    `json:"tool_name"`
	Actor      *string   `json:"actor,omitempty"`
	Role       *string   `json:"role,omitempty"`
	Scopes     []string  `json:"scopes,omitempty"`
	Decision   string    `json:"decision"`
	Status     string    `json:"status"`
	Reason     *string   `json:"reason,omitempty"`
	LatencyMS  int       `json:"latency_ms"`
	IdempotencyPresent bool              `json:"idempotency_present,omitempty"`
	IdempotencyOutcome string            `json:"idempotency_outcome,omitempty"`
	TimeoutMS          *int              `json:"timeout_ms,omitempty"`
	BudgetRemainingMS  *int              `json:"budget_remaining_ms_at_execute,omitempty"`
	StageDurationsMS   map[string]int64  `json:"stage_durations_ms,omitempty"`
	PrevEventHash      *string           `json:"prev_event_hash,omitempty"`
	EventHash          *string           `json:"event_hash,omitempty"`
	HashAlgo           string            `json:"hash_algo,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	Input      any       `json:"input"`
	Context    any       `json:"context"`
	DLPSummary any       `json:"dlp_summary,omitempty"`
	Output     any       `json:"output,omitempty"`
	Error      *ErrorObj `json:"error,omitempty"`
}

type ErrorObj struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ListAuditResponse struct {
	Items []AuditItem `json:"items"`
}
