package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProposalStatus string

const (
	ProposalStatusProposed         ProposalStatus = "proposed"
	ProposalStatusDryRunOK         ProposalStatus = "dry_run_ok"
	ProposalStatusDryRunFailed     ProposalStatus = "dry_run_failed"
	ProposalStatusAwaitingApproval ProposalStatus = "awaiting_approval"
	ProposalStatusApplied          ProposalStatus = "applied"
	ProposalStatusFailed           ProposalStatus = "failed"
	ProposalStatusRolledBack       ProposalStatus = "rolled_back"
)

type ExecutionMode string

const (
	ExecutionModeDryRun   ExecutionMode = "dry_run"
	ExecutionModeApply    ExecutionMode = "apply"
	ExecutionModeRollback ExecutionMode = "rollback"
)

type ExecutionStatus string

const (
	ExecutionStatusOK     ExecutionStatus = "ok"
	ExecutionStatusFailed ExecutionStatus = "failed"
)

type CatalogItem struct {
	ActionType       string
	Schema           map[string]any
	RequiresApproval bool
	MaxTTLSeconds    int
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Proposal struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	IncidentID       *uuid.UUID
	ActionType       string
	Scope            map[string]any
	Params           map[string]any
	TTLSeconds       int
	EvidenceRefs     []string
	IdempotencyKey   string
	Status           ProposalStatus
	ApprovalRequired bool
	ProposedBy       *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Execution struct {
	ID          uuid.UUID
	ProposalID  uuid.UUID
	OrgID       uuid.UUID
	Mode        ExecutionMode
	Status      ExecutionStatus
	ErrorCode   *string
	ErrorMessage *string
	Output      map[string]any
	ExecutedBy  *string
	ExecutedAt  time.Time
}

type Approval struct {
	ID         uuid.UUID
	ProposalID uuid.UUID
	OrgID      uuid.UUID
	Approved   bool
	Approver   string
	Comment    *string
	CreatedAt  time.Time
}
