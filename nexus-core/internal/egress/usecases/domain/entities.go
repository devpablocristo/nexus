package domain

import (
	"time"

	"github.com/google/uuid"
)

type Rule struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	ToolID    uuid.UUID
	Host      string
	Enabled   bool
	CreatedAt time.Time
}
