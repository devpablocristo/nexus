package dto

import "time"

type ApprovalItem struct {
	ID              string         `json:"id"`
	IntentID        *string        `json:"intent_id,omitempty"`
	RequestID       string         `json:"request_id"`
	ToolName        string         `json:"tool_name"`
	Actor           *string        `json:"actor,omitempty"`
	Role            *string        `json:"role,omitempty"`
	InputRedacted   map[string]any `json:"input_redacted"`
	ContextRedacted map[string]any `json:"context_redacted"`
	Reason          string         `json:"reason"`
	Status          string         `json:"status"`
	DecidedBy       *string        `json:"decided_by,omitempty"`
	DecidedAt       *time.Time     `json:"decided_at,omitempty"`
	ExpiresAt       time.Time      `json:"expires_at"`
	CreatedAt       time.Time      `json:"created_at"`
}

type ListApprovalsResponse struct {
	Items []ApprovalItem `json:"items"`
}

type DecideRequest struct {
	DecidedBy string `json:"decided_by"`
}
