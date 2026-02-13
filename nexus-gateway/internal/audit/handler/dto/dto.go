package dto

import "time"

type AuditItem struct {
	RequestID string    `json:"request_id"`
	ToolName  string    `json:"tool_name"`
	Actor     *string   `json:"actor,omitempty"`
	Decision  string    `json:"decision"`
	Status    string    `json:"status"`
	Reason    *string   `json:"reason,omitempty"`
	LatencyMS int       `json:"latency_ms"`
	CreatedAt time.Time `json:"created_at"`
	Input     any       `json:"input"`
	Context   any       `json:"context"`
	Output    any       `json:"output,omitempty"`
	Error     *ErrorObj `json:"error,omitempty"`
}

type ErrorObj struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ListAuditResponse struct {
	Items []AuditItem `json:"items"`
}
