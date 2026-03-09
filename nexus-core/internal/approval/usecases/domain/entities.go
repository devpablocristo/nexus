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
	StatusExpired  Status = "expired"
)

type PendingApproval struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	ToolID          uuid.UUID
	IntentID        *uuid.UUID
	RequestID       string
	ToolName        string
	Actor           *string
	Role            *string
	InputRedacted   map[string]any
	ContextRedacted map[string]any
	Reason          string
	PolicyID        *uuid.UUID
	Status          Status
	DecidedBy       *string
	DecidedAt       *time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type CreateRequest struct {
	OrgID           uuid.UUID
	ToolID          uuid.UUID
	IntentID        *uuid.UUID
	RequestID       string
	ToolName        string
	Actor           *string
	Role            *string
	InputRedacted   map[string]any
	ContextRedacted map[string]any
	Reason          string
	PolicyID        *uuid.UUID
	TTLSeconds      int
}
