package dto

import (
	"encoding/json"
	"time"
)

type CreateActionRequest struct {
	ActionType    string          `json:"action_type"`
	ResourceID    string          `json:"resource_id"`
	ResourceType  string          `json:"resource_type"`
	SourceSystem  string          `json:"source_system"`
	Justification string          `json:"justification"`
	RequestedBy   ActorRef        `json:"requested_by"`
	ProposedBy    ActorRef        `json:"proposed_by"`
	Payload       json.RawMessage `json:"payload"`
	Metadata      map[string]any  `json:"metadata"`
}

type ActorRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type DecideActionRequest struct {
	DecidedBy ActorRef `json:"decided_by"`
	Comment   string   `json:"comment,omitempty"`
}

type ExecuteActionRequest struct {
	LeaseID    string   `json:"lease_id"`
	ExecutedBy ActorRef `json:"executed_by"`
}

type ActionResponse struct {
	ID              string             `json:"id"`
	ActionType      string             `json:"action_type"`
	Status          string             `json:"status"`
	Decision        string             `json:"decision"`
	ResourceID      string             `json:"resource_id"`
	ResourceType    string             `json:"resource_type"`
	SourceSystem    string             `json:"source_system"`
	Justification   string             `json:"justification"`
	RequestedBy     ActorRef           `json:"requested_by"`
	ProposedBy      ActorRef           `json:"proposed_by"`
	Payload         json.RawMessage    `json:"payload"`
	Metadata        map[string]any     `json:"metadata,omitempty"`
	Risk            RiskResponse       `json:"risk"`
	EvidenceSummary EvidenceSummary    `json:"evidence_summary"`
	Approval        *ApprovalResponse  `json:"approval"`
	Lease           *LeaseResponse     `json:"lease"`
	Execution       *ExecutionResponse `json:"execution"`
	ExpiresAt       time.Time          `json:"expires_at"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type ListActionsResponse struct {
	Items []ActionResponse `json:"items"`
}

type RiskResponse struct {
	Level   string               `json:"level"`
	Score   int                  `json:"score"`
	Summary string               `json:"summary"`
	Factors []RiskFactorResponse `json:"factors"`
}

type RiskFactorResponse struct {
	Code    string `json:"code"`
	Summary string `json:"summary"`
	Weight  int    `json:"weight"`
}

type EvidenceSummary struct {
	Status       string `json:"status"`
	ChecksTotal  int    `json:"checks_total"`
	ChecksPassed int    `json:"checks_passed"`
	ChecksFailed int    `json:"checks_failed"`
}

type ApprovalResponse struct {
	Required      bool       `json:"required"`
	ApprovalID    *string    `json:"approval_id"`
	Status        string     `json:"status"`
	RequiredCount int        `json:"required_count"`
	GrantedCount  int        `json:"granted_count"`
	DecidedBy     *ActorRef  `json:"decided_by,omitempty"`
	Comment       string     `json:"comment,omitempty"`
	ExpiresAt     time.Time  `json:"expires_at"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type LeaseResponse struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	Scope     LeaseScope `json:"scope"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type LeaseScope struct {
	ActionID     string `json:"action_id"`
	ActionType   string `json:"action_type"`
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
}

type ExecutionResponse struct {
	Status     string         `json:"status"`
	ExecutedBy ActorRef       `json:"executed_by"`
	Result     map[string]any `json:"result"`
	ExecutedAt time.Time      `json:"executed_at"`
}

type EvidenceRecordResponse struct {
	ID        string         `json:"id"`
	ActionID  string         `json:"action_id"`
	Kind      string         `json:"kind"`
	Status    string         `json:"status"`
	Summary   string         `json:"summary"`
	Details   map[string]any `json:"details"`
	CreatedAt time.Time      `json:"created_at"`
}

type EvidenceListResponse struct {
	Items []EvidenceRecordResponse `json:"items"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
