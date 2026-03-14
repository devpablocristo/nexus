package domain

import (
	"time"

	"github.com/google/uuid"
)

type RiskClass string

const (
	RiskClassRead            RiskClass = "read"
	RiskClassPlan            RiskClass = "plan"
	RiskClassMutateNonProd   RiskClass = "mutate_non_prod"
	RiskClassMutateProd      RiskClass = "mutate_prod"
	RiskClassDestructiveProd RiskClass = "destructive_prod"
	RiskClassBreakGlass      RiskClass = "break_glass"
)

type PreflightStatus string

const (
	PreflightStatusNotRequired PreflightStatus = "not_required"
	PreflightStatusPassed      PreflightStatus = "passed"
	PreflightStatusFailed      PreflightStatus = "failed"
)

type IntentStatus string

const (
	IntentStatusPendingApproval IntentStatus = "pending_approval"
	IntentStatusApproved        IntentStatus = "approved"
	IntentStatusRejected        IntentStatus = "rejected"
	IntentStatusExecuted        IntentStatus = "executed"
)

type ExecutionIntent struct {
	ID                   uuid.UUID
	ToolID               string
	ToolName             string
	RequestID            string
	Input                map[string]any
	Context              map[string]any
	PolicyID             *uuid.UUID
	RiskClass            RiskClass
	Reason               string
	Status               IntentStatus
	PreflightStatus      PreflightStatus
	PreflightSummary     map[string]any
	PreflightCompletedAt *time.Time
	ApprovalID           *uuid.UUID
	ExpiresAt            time.Time
	ExecutedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PreflightReview struct {
	IntentID     uuid.UUID
	ToolName     string
	RiskClass    RiskClass
	Reason       string
	Status       PreflightStatus
	Summary      map[string]any
	CompletedAt  *time.Time
	ApprovalID   *uuid.UUID
	IntentStatus IntentStatus
}

type ExecutionLeaseStatus string

const (
	ExecutionLeaseStatusActive  ExecutionLeaseStatus = "active"
	ExecutionLeaseStatusUsed    ExecutionLeaseStatus = "used"
	ExecutionLeaseStatusExpired ExecutionLeaseStatus = "expired"
)

type ExecutionLease struct {
	ID              uuid.UUID
	IntentID        uuid.UUID
	ToolName        string
	RiskClass       RiskClass
	Status          ExecutionLeaseStatus
	CredentialMode  string
	CredentialHints map[string]any
	ExpiresAt       time.Time
	UsedAt          *time.Time
	CreatedAt       time.Time
}
