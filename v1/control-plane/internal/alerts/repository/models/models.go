package models

import (
	"time"

	"github.com/google/uuid"
)

type AlertRule struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrgID           uuid.UUID  `gorm:"type:uuid;not null"`
	Name            string     `gorm:"not null"`
	Metric          string     `gorm:"not null"`
	Threshold       float64    `gorm:"not null"`
	WindowSeconds   int        `gorm:"default:300"`
	ToolName        *string
	WebhookURL      string     `gorm:"not null"`
	CooldownSeconds int        `gorm:"default:300"`
	Enabled         bool       `gorm:"default:true"`
	LastFiredAt     *time.Time
	CreatedAt       time.Time  `gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime"`
}

func (AlertRule) TableName() string { return "alert_rules" }
