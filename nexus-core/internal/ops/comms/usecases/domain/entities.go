package domain

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft            Status = "draft"
	StatusAwaitingApproval Status = "awaiting_approval"
	StatusSentInternal     Status = "sent_internal"
	StatusSentExternal     Status = "sent_external"
)

type Draft struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	IncidentID       *uuid.UUID
	Channel          string
	Audience         string
	Status           Status
	Content          map[string]any
	RequiresApproval bool
	CreatedBy        *string
	CreatedAt        time.Time
	SentAt           *time.Time
}
