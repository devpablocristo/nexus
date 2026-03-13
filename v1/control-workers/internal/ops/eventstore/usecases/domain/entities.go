package domain

import (
	"time"

	"github.com/google/uuid"
)

type Correlation struct {
	RequestID  *string `json:"request_id,omitempty"`
	IncidentID *string `json:"incident_id,omitempty"`
	ActionID   *string `json:"action_id,omitempty"`
}

type Actor struct {
	ActorID   *string `json:"actor_id,omitempty"`
	ActorType string  `json:"actor_type"`
}

type Envelope struct {
	ID          uuid.UUID     `json:"id"`
	EventType   string        `json:"event_type"`
	Version     int           `json:"version"`
	OccurredAt  time.Time     `json:"occurred_at"`
	OrgID       uuid.UUID     `json:"org_id"`
	Correlation Correlation   `json:"correlation"`
	Actor       Actor         `json:"actor"`
	Source      string        `json:"source"`
	Payload     map[string]any `json:"payload"`
}

type StoredEvent struct {
	Sequence        int64
	Envelope        Envelope
	SchemaValid     bool
	ValidationError *string
	CreatedAt       time.Time
}

type EventContract struct {
	EventType string
	Version   int
	Schema    map[string]any
	Enabled   bool
	CreatedBy *string
	CreatedAt time.Time
}
