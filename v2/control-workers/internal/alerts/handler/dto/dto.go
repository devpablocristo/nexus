package dto

import "time"

type CreateAlertRequest struct {
	SourceKind string         `json:"source_kind"`
	SourceID   string         `json:"source_id"`
	ActionID   string         `json:"action_id,omitempty"`
	ResourceID string         `json:"resource_id,omitempty"`
	ResourceType string       `json:"resource_type,omitempty"`
	Channel    string         `json:"channel"`
	Route      string         `json:"route"`
	Severity   string         `json:"severity"`
	Status     string         `json:"status,omitempty"`
	Summary    string         `json:"summary"`
	Body       string         `json:"body"`
	Details    map[string]any `json:"details"`
}

type UpdateAlertRequest struct {
	Status  *string        `json:"status"`
	Summary *string        `json:"summary"`
	Body    *string        `json:"body"`
	Details map[string]any `json:"details"`
}

type AlertResponse struct {
	ID         string         `json:"id"`
	SourceKind string         `json:"source_kind"`
	SourceID   string         `json:"source_id"`
	IncidentID string         `json:"incident_id,omitempty"`
	ActionID   string         `json:"action_id,omitempty"`
	ResourceID string         `json:"resource_id,omitempty"`
	ResourceType string       `json:"resource_type,omitempty"`
	Channel    string         `json:"channel"`
	Route      string         `json:"route"`
	Severity   string         `json:"severity"`
	Status     string         `json:"status"`
	Summary    string         `json:"summary"`
	Body       string         `json:"body"`
	Details    map[string]any `json:"details"`
	Archived   bool           `json:"archived"`
	ArchivedAt *time.Time     `json:"archived_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type ListAlertsResponse struct {
	Items []AlertResponse `json:"items"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
