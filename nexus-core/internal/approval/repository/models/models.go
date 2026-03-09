package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type PendingApproval struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrgID           uuid.UUID  `gorm:"type:uuid;not null"`
	ToolID          uuid.UUID  `gorm:"type:uuid;not null"`
	IntentID        *uuid.UUID `gorm:"type:uuid"`
	RequestID       string     `gorm:"not null"`
	ToolName        string     `gorm:"not null"`
	Actor           *string
	Role            *string
	InputRedacted   datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	ContextRedacted datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	Reason          string         `gorm:"default:''"`
	PolicyID        *uuid.UUID     `gorm:"type:uuid"`
	Status          string         `gorm:"default:'pending'"`
	DecidedBy       *string
	DecidedAt       *time.Time
	ExpiresAt       time.Time `gorm:"not null"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
}

func (PendingApproval) TableName() string { return "pending_approvals" }
