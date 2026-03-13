package dto

import "time"

type ApprovalItem struct {
	ID        string     `json:"id"`
	IntentID  *string    `json:"intent_id,omitempty"`
	RequestID string     `json:"request_id"`
	ToolName  string     `json:"tool_name"`
	Reason    string     `json:"reason"`
	Status    string     `json:"status"`
	DecidedBy *string    `json:"decided_by,omitempty"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type ListApprovalsResponse struct {
	Items []ApprovalItem `json:"items"`
}

type DecideRequest struct {
	DecidedBy string `json:"decided_by"`
}

type DecideResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
