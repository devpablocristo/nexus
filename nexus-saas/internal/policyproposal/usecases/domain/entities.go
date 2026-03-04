package domain

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusShadow   Status = "shadow"
)

type Proposal struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Status         Status
	Diff           map[string]any
	Rationale      string
	TestsSuggested []string
	RollbackPlan   string
	CreatedBy      *string
	CreatedAt      time.Time
	DecidedBy      *string
	DecidedAt      *time.Time
}
