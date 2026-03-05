package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExternalID string    `gorm:"uniqueIndex"`
	Email      string    `gorm:"uniqueIndex"`
	Name       string
	AvatarURL  *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (User) TableName() string { return "users" }

type OrgMember struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID    uuid.UUID `gorm:"type:uuid;index"`
	UserID   uuid.UUID `gorm:"type:uuid;index"`
	Role     string
	JoinedAt time.Time
}

func (OrgMember) TableName() string { return "org_members" }
