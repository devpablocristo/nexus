package domain

import (
	"time"

	"github.com/google/uuid"
)

type IntentStatus string

const (
	IntentStatusPendingApproval IntentStatus = "pending_approval"
	IntentStatusApproved        IntentStatus = "approved"
	IntentStatusRejected        IntentStatus = "rejected"
	IntentStatusExecuted        IntentStatus = "executed"
)

type ExecutionIntent struct {
	ID         uuid.UUID
	ToolID     string
	ToolName   string
	RequestID  string
	Input      map[string]any
	Context    map[string]any
	PolicyID   *uuid.UUID
	Reason     string
	Status     IntentStatus
	ApprovalID *uuid.UUID
	ExpiresAt  time.Time
	ExecutedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
