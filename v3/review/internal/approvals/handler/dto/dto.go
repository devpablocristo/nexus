package dto

// ApprovalDecisionRequest representa el body de approve/reject.
type ApprovalDecisionRequest struct {
	DecidedBy string `json:"decided_by"`
	Note      string `json:"note,omitempty"`
}

// ApprovalResponse representa una approval en la respuesta HTTP.
type ApprovalResponse struct {
	ID           string  `json:"id"`
	RequestID    string  `json:"request_id"`
	Status       string  `json:"status"`
	DecidedBy    string  `json:"decided_by,omitempty"`
	DecisionNote string  `json:"decision_note,omitempty"`
	DecidedAt    *string `json:"decided_at,omitempty"`
	ExpiresAt    string  `json:"expires_at"`
	CreatedAt    string  `json:"created_at"`
}
