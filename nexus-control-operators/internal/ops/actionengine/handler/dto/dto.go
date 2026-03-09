// Package dto defines HTTP request/response payloads for actionengine.
package dto

type ActionEngineRequest struct {
	IncidentID      *string           `json:"incident_id,omitempty"`
	ProposalID      *string           `json:"proposal_id,omitempty"`
	ActionType      string            `json:"action_type,omitempty"`
	Scope           map[string]any    `json:"scope,omitempty"`
	TTLSeconds      int               `json:"ttl_seconds,omitempty"`
	Params          map[string]any    `json:"params,omitempty"`
	EvidenceRefs    []string          `json:"evidence_refs,omitempty"`
	LeaseHeaders    map[string]string `json:"lease_headers,omitempty"`
	ApprovalGranted bool              `json:"approval_granted,omitempty"`
	ApprovalComment *string           `json:"approval_comment,omitempty"`
}

type ActionEngineResponse struct {
	RequestID        string         `json:"request_id,omitempty"`
	ProposalID       string         `json:"proposal_id"`
	ExecutionID      *string        `json:"execution_id,omitempty"`
	Status           string         `json:"status"`
	ActionType       string         `json:"action_type"`
	IdempotencyKey   string         `json:"idempotency_key"`
	ScopeHash        string         `json:"scope_hash,omitempty"`
	ParamsHash       string         `json:"params_hash,omitempty"`
	ApprovalRequired bool           `json:"approval_required"`
	Replay           bool           `json:"replay"`
	Scope            map[string]any `json:"scope,omitempty"`
	Params           map[string]any `json:"params,omitempty"`
}
