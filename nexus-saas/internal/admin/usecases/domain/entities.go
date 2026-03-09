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

const (
	ProtectedResourceMatchExact    = "exact"
	ProtectedResourceMatchContains = "contains"
)

type ProtectedResource struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Name         string
	ResourceType string
	MatchValue   string
	MatchMode    string
	Environment  string
	Reason       string
	Enabled      bool
	CreatedBy    *string
	UpdatedBy    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const (
	RestoreEvidenceStatusPassed = "passed"
	RestoreEvidenceStatusFailed = "failed"
)

type RestoreEvidence struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Environment    string
	System         string
	Status         string
	SnapshotID     string
	RestoreTarget  string
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Source         string
	ArtifactSHA256 string
	Summary        map[string]any
	CreatedAt      time.Time
}
