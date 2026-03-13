package domain

import (
	"time"

	"github.com/google/uuid"
)

type AgentSession struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	SessionID    string
	Actor        *string
	TotalCalls   int
	TotalWrites  int
	TotalDenials int
	Metadata     map[string]any
	CreatedAt    time.Time
	LastCallAt   time.Time
}

type SessionLimits struct {
	MaxCallsPerSession  int
	MaxWritesPerSession int
}
