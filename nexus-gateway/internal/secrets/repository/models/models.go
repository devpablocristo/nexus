package models

import (
	"time"

	"github.com/google/uuid"
)

type ToolSecret struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID `gorm:"type:uuid;index"`
	ToolID     uuid.UUID `gorm:"type:uuid;index"`
	SecretType string
	KeyName    string
	Ciphertext []byte
	Nonce      []byte
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
