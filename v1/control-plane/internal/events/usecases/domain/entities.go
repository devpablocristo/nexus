package domain

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID        int64
	OrgID     uuid.UUID
	EventType string
	Payload   map[string]any
	CreatedAt time.Time
}
