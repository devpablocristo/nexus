package models

import (
	"time"

	"github.com/google/uuid"
)

type NotificationPreference struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID           uuid.UUID `gorm:"type:uuid;index"`
	NotificationType string
	Channel          string
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (NotificationPreference) TableName() string { return "notification_preferences" }

type NotificationLog struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID            uuid.UUID  `gorm:"type:uuid;index"`
	UserID           *uuid.UUID `gorm:"type:uuid;index"`
	NotificationType string
	Channel          string
	Recipient        string
	Subject          string
	Status           string
	DedupKey         *string
	ErrorMessage     *string
	CreatedAt        time.Time
}

func (NotificationLog) TableName() string { return "notification_log" }
