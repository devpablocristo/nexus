package dto

type ApplyActionRequest struct {
	ScopeType    string         `json:"scope_type" binding:"required"`
	ScopeID      *string        `json:"scope_id,omitempty"`
	ActionType   string         `json:"action_type" binding:"required"`
	Params       map[string]any `json:"params"`
	TTLSeconds   int            `json:"ttl_seconds"`
	EvidenceRefs []string       `json:"evidence_refs"`
}

type RollbackActionRequest struct {
	ActionID string `json:"action_id" binding:"required"`
}

type ActionItem struct {
	ID           string         `json:"id"`
	ScopeType    string         `json:"scope_type"`
	ScopeID      *string        `json:"scope_id,omitempty"`
	ActionType   string         `json:"action_type"`
	Params       map[string]any `json:"params"`
	TTLSeconds   int            `json:"ttl_seconds"`
	Status       string         `json:"status"`
	EvidenceRefs []string       `json:"evidence_refs"`
	CreatedBy    *string        `json:"created_by,omitempty"`
	CreatedAt    string         `json:"created_at"`
	RolledBackAt *string        `json:"rolled_back_at,omitempty"`
	RolledBackBy *string        `json:"rolled_back_by,omitempty"`
}

type ListActionsResponse struct {
	Items []ActionItem `json:"items"`
}
