package domain

import (
	"time"

	"github.com/google/uuid"
)

type Approval struct {
	ID           uuid.UUID
	RequestID    uuid.UUID
	Status       string
	DecidedBy    string
	DecisionNote string
	DecidedAt    *time.Time
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

const (
	ApprovalStatusPending   = "pending"
	ApprovalStatusApproved  = "approved"
	ApprovalStatusRejected  = "rejected"
	ApprovalStatusExpired   = "expired"
)
