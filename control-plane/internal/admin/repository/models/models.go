package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type TenantSettings struct {
	OrgID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	PlanCode   string
	Status     string
	DeletedAt  *time.Time
	HardLimits datatypes.JSON `gorm:"column:hard_limits_json;type:jsonb"`
	UpdatedBy  *string
	UpdatedAt  time.Time
	CreatedAt  time.Time
}

type AdminActivityEvent struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index"`
	Actor        *string
	Action       string
	ResourceType string
	ResourceID   *string
	Payload      datatypes.JSON `gorm:"column:payload_json;type:jsonb"`
	CreatedAt    time.Time
}

type ProtectedResource struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index"`
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

type RestoreEvidence struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID          uuid.UUID `gorm:"type:uuid;index"`
	Environment    string
	System         string
	Status         string
	SnapshotID     string
	RestoreTarget  string
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Source         string
	ArtifactSHA256 string
	Summary        datatypes.JSON `gorm:"column:summary_json;type:jsonb"`
	CreatedAt      time.Time
}
