package domain

import (
	"time"

	"github.com/google/uuid"
)

// ApprovalStatus representa el estado de una approval.
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

type Approval struct {
	ID           uuid.UUID
	RequestID    uuid.UUID
	Status       ApprovalStatus
	DecidedBy    string
	DecisionNote string
	DecidedAt    *time.Time
	ExpiresAt    time.Time
	CreatedAt    time.Time
}
