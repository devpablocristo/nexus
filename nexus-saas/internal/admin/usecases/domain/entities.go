package domain

import (
	"time"

	"github.com/google/uuid"
)

type TenantSettings struct {
	OrgID      uuid.UUID
	PlanCode   string
	Status     string
	DeletedAt  *time.Time
	HardLimits map[string]any
	UpdatedBy  *string
	UpdatedAt  time.Time
	CreatedAt  time.Time
}

const (
	TenantStatusActive    = "active"
	TenantStatusSuspended = "suspended"
	TenantStatusDeleted   = "deleted"
)

type AdminActivityEvent struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Actor        *string
	Action       string
	ResourceType string
	ResourceID   *string
	Payload      map[string]any
	CreatedAt    time.Time
}
