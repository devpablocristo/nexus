package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Tool struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID            uuid.UUID `gorm:"type:uuid;index:,unique,priority:1"`
	Name             string    `gorm:"index:,unique,priority:2"`
	Kind             string
	Description      *string
	Method           string
	URL              string
	InputSchemaJSON  datatypes.JSON `gorm:"type:jsonb"`
	OutputSchemaJSON datatypes.JSON `gorm:"type:jsonb"`
	ActionType       string
	Classification   string
	Sensitivity      string
	RiskLevel        int
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
