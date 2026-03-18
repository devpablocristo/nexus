package domain

import (
	"time"

	"github.com/google/uuid"
)

type Approval struct {
	ID           uuid.UUID  `json:"id"`
	RequestID    uuid.UUID  `json:"request_id"`
	Status       string     `json:"status"`
	DecidedBy    string     `json:"decided_by,omitempty"`
	DecisionNote string     `json:"decision_note,omitempty"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	ExpiresAt    time.Time  `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
	ApprovalStatusExpired  = "expired"
)
