package models

import (
	"time"

	"github.com/google/uuid"
)

type Org struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string
	CreatedAt time.Time
}

type OrgAPIKey struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID `gorm:"type:uuid;index"`
	APIKeyHash string    `gorm:"uniqueIndex"`
	Name       string
	CreatedAt  time.Time
}
