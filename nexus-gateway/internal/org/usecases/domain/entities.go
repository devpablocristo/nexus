package domain

import (
	"time"

	"github.com/google/uuid"
)

type Org struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

type APIKey struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	APIKeyHash string
	Name       string
	CreatedAt  time.Time
}
