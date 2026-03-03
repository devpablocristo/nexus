package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AgentSession struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrgID        uuid.UUID      `gorm:"type:uuid;not null"`
	SessionID    string         `gorm:"not null"`
	Actor        *string
	TotalCalls   int            `gorm:"default:0"`
	TotalWrites  int            `gorm:"default:0"`
	TotalDenials int            `gorm:"default:0"`
	Metadata     datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	LastCallAt   time.Time      `gorm:"autoCreateTime"`
}

func (AgentSession) TableName() string { return "agent_sessions" }
