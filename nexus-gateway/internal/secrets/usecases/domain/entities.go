package domain

import (
	"time"

	"github.com/google/uuid"
)

type ToolSecret struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	ToolID         uuid.UUID
	SecretType     string
	KeyName        string
	PlaintextValue string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
