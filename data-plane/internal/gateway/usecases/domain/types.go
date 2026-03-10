// Package domain defines the gateway runtime contracts.
package domain

import (
	"time"

	"github.com/google/uuid"
)

type RunRequest struct {
	RequestID        string
	ToolName         string
	ToolID           string
	IntentID         string
	ExecutionLease   *ExecutionLease
	Input            map[string]any
	Context          map[string]any
	Actor            *string
	Role             *string
	Scopes           []string
	IdempotencyKey   *string
	TimeoutMS        int
	RequestSource    string
	AuthMethod       string
	StageDurations   map[string]int64
	TimeoutAtExecute int
	Idempotency      IdempotencyMeta
}

type RunStatus string

const (
	RunStatusSuccess RunStatus = "success"
	RunStatusError   RunStatus = "error"
	RunStatusBlocked RunStatus = "blocked"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

type RunResponse struct {
	RequestID   string
	Decision    Decision
	ToolName    string
	Status      RunStatus
	Reason      *string
	Result      any
	ErrorCode   *string
	ErrorMsg    *string
	LatencyMS   int64
	HTTPStatus  int
	Idempotency IdempotencyMeta
	IntentID    *string
	ApprovalID  *string
	RiskClass   *string
	LeaseID     *string
}

type SimulateResponse struct {
	RequestID  string
	Decision   Decision
	ToolName   string
	Status     RunStatus
	Reason     *string
	ErrorCode  *string
	ErrorMsg   *string
	LatencyMS  int64
	HTTPStatus int
	Explain    map[string]any
}

type IdempotencyOutcome string

const (
	IdempotencyNew             IdempotencyOutcome = "NEW"
	IdempotencyReplay          IdempotencyOutcome = "REPLAY"
	IdempotencyInProgress      IdempotencyOutcome = "IN_PROGRESS"
	IdempotencyConflict        IdempotencyOutcome = "CONFLICT"
	IdempotencyMissingRequired IdempotencyOutcome = "MISSING_REQUIRED"
	IdempotencySkippedNotWrite IdempotencyOutcome = "SKIPPED_NOT_WRITE"
)

type IdempotencyMeta struct {
	Present bool
	Outcome IdempotencyOutcome
}

type RiskClass string

const (
	RiskClassRead            RiskClass = "read"
	RiskClassPlan            RiskClass = "plan"
	RiskClassMutateNonProd   RiskClass = "mutate_nonprod"
	RiskClassMutateProd      RiskClass = "mutate_prod"
	RiskClassDestructiveProd RiskClass = "destructive_prod"
	RiskClassBreakGlass      RiskClass = "break_glass"
)

type IntentStatus string

const (
	IntentStatusPendingApproval IntentStatus = "pending_approval"
	IntentStatusApproved        IntentStatus = "approved"
	IntentStatusRejected        IntentStatus = "rejected"
	IntentStatusExecuted        IntentStatus = "executed"
	IntentStatusExpired         IntentStatus = "expired"
)

type PreflightStatus string

const (
	PreflightStatusNotRequired PreflightStatus = "not_required"
	PreflightStatusPassed      PreflightStatus = "passed"
	PreflightStatusFailed      PreflightStatus = "failed"
)

type ExecutionIntent struct {
	ID                   uuid.UUID
	OrgID                uuid.UUID
	ToolID               uuid.UUID
	ToolName             string
	RequestID            string
	Actor                *string
	Role                 *string
	Scopes               []string
	Input                map[string]any
	Context              map[string]any
	PolicyID             *uuid.UUID
	RiskClass            RiskClass
	Reason               string
	ApprovalID           *uuid.UUID
	Status               IntentStatus
	PreflightStatus      PreflightStatus
	PreflightSummary     map[string]any
	PreflightArtifactSHA *string
	PreflightCompletedAt *time.Time
	ExpiresAt            time.Time
	ApprovedAt           *time.Time
	ExecutedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PreflightReview struct {
	IntentID       uuid.UUID
	ToolName       string
	RiskClass      RiskClass
	Reason         string
	Status         PreflightStatus
	Summary        map[string]any
	ArtifactSHA256 *string
	CompletedAt    *time.Time
	ApprovalID     *uuid.UUID
	IntentStatus   IntentStatus
}

type ExecutionLeaseStatus string

const (
	ExecutionLeaseStatusActive  ExecutionLeaseStatus = "active"
	ExecutionLeaseStatusUsed    ExecutionLeaseStatus = "used"
	ExecutionLeaseStatusExpired ExecutionLeaseStatus = "expired"
	ExecutionLeaseStatusRevoked ExecutionLeaseStatus = "revoked"
)

type ExecutionLease struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
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
