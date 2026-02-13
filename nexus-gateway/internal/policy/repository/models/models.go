package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Policy struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID          uuid.UUID `gorm:"type:uuid;index"`
	ToolID         uuid.UUID `gorm:"type:uuid;index"`
	Effect         string
	Priority       int
	ConditionsJSON datatypes.JSON `gorm:"type:jsonb"`
	LimitsJSON     datatypes.JSON `gorm:"type:jsonb"`
	ReasonTemplate string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
