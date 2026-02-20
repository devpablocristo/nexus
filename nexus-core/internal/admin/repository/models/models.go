package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type TenantSettings struct {
	OrgID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	PlanCode   string
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
