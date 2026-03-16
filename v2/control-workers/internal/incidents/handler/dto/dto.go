package dto

import "time"

type CreateIncidentRequest struct {
	SourceKind   string         `json:"source_kind"`
	SourceID     string         `json:"source_id"`
	ActionType   string         `json:"action_type"`
	ResourceID   string         `json:"resource_id"`
	ResourceType string         `json:"resource_type"`
	Trigger      string         `json:"trigger"`
	RiskLevel    string         `json:"risk_level"`
	Summary      string         `json:"summary,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	Details      map[string]any `json:"details"`
}

type UpdateIncidentRequest struct {
	Status  *string        `json:"status"`
	Summary *string        `json:"summary"`
	Reason  *string        `json:"reason"`
	Details map[string]any `json:"details"`
}

type IncidentResponse struct {
	ID           string         `json:"id"`
	SourceKind   string         `json:"source_kind"`
	SourceID     string         `json:"source_id"`
	ActionID     string         `json:"action_id,omitempty"`
	ActionType   string         `json:"action_type"`
	ResourceID   string         `json:"resource_id"`
	ResourceType string         `json:"resource_type"`
	Trigger      string         `json:"trigger"`
	RiskLevel    string         `json:"risk_level"`
	Severity     string         `json:"severity"`
	Status       string         `json:"status"`
	Summary      string         `json:"summary"`
	Reason       string         `json:"reason,omitempty"`
	Details      map[string]any `json:"details"`
	Archived     bool           `json:"archived"`
	ArchivedAt   *time.Time     `json:"archived_at,omitempty"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type ListIncidentsResponse struct {
	Items []IncidentResponse `json:"items"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
