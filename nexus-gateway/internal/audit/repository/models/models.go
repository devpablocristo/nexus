package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AuditEvent struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID `gorm:"type:uuid;index"`
	ToolID          uuid.UUID `gorm:"type:uuid;index"`
	ToolName        string    `gorm:"index"`
	RequestID       string    `gorm:"uniqueIndex"`
	Actor           *string
	InputRedacted   datatypes.JSON `gorm:"type:jsonb"`
	ContextRedacted datatypes.JSON `gorm:"type:jsonb"`
	Decision        string
	PolicyID        *uuid.UUID `gorm:"type:uuid"`
	Reason          *string
	Status          string
	OutputRedacted  datatypes.JSON `gorm:"type:jsonb"`
	ErrorCode       *string
	ErrorMessage    *string
	LatencyMS       int
	CreatedAt       time.Time
}
