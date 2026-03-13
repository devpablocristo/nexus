package domain

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

type PendingApproval struct {
	ID        uuid.UUID
	IntentID  *uuid.UUID
	RequestID string
	ToolName  string
	Reason    string
	Status    Status
	DecidedBy *string
	DecidedAt *time.Time
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateRequest struct {
	IntentID   *uuid.UUID
	RequestID  string
	ToolName   string
	Reason     string
	TTLSeconds int
}
