package dto

import (
	"time"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
)

type AuditResponse struct {
	ID            string             `json:"id"`
	EventType     string             `json:"event_type"`
	SourceService string             `json:"source_service"`
	ActionID      string             `json:"action_id,omitempty"`
	ResourceID    string             `json:"resource_id,omitempty"`
	ResourceType  string             `json:"resource_type,omitempty"`
	Actor         *sharedaudit.Actor `json:"actor,omitempty"`
	Summary       string             `json:"summary"`
	Data          map[string]any     `json:"data,omitempty"`
	OccurredAt    time.Time          `json:"occurred_at"`
	CreatedAt     time.Time          `json:"created_at"`
}

type ListAuditResponse struct {
	Items []AuditResponse `json:"items"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}
