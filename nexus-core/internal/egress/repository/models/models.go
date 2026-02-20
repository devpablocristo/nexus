package models

import (
	"time"

	"github.com/google/uuid"
)

type Rule struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;index"`
	ToolID    uuid.UUID `gorm:"type:uuid;index"`
	Host      string
	Enabled   bool
	CreatedAt time.Time
}

func (Rule) TableName() string { return "tool_egress_rules" }
