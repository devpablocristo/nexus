package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ActionType identifies the kind of protected action proposed to Nexus.
type ActionType string

const (
	ActionTypeWithdrawal       ActionType = "withdrawal"
	ActionTypeTreasuryTransfer ActionType = "treasury_transfer"
	ActionTypeHotToColdMove    ActionType = "hot_to_cold_move"
)

// ActionStatus tracks the lifecycle of a protected action.
type ActionStatus string

const (
	ActionStatusPending         ActionStatus = "pending"
	ActionStatusBlocked         ActionStatus = "blocked"
	ActionStatusPendingApproval ActionStatus = "pending_approval"
	ActionStatusApproved        ActionStatus = "approved"
	ActionStatusLeased          ActionStatus = "leased"
	ActionStatusExecuted        ActionStatus = "executed"
	ActionStatusRejected        ActionStatus = "rejected"
	ActionStatusExpired         ActionStatus = "expired"
)

// Decision is the deterministic outcome produced by the core.
type Decision string

const (
	DecisionAllow           Decision = "allow"
	DecisionDeny            Decision = "deny"
	DecisionRequireApproval Decision = "require_approval"
)

// ResourceType identifies the protected surface tied to an action.
type ResourceType string

const (
	ResourceTypeWallet   ResourceType = "wallet"
	ResourceTypeTreasury ResourceType = "treasury"
	ResourceTypeVault    ResourceType = "vault"
)

// ActorType identifies who proposed or approved an action.
type ActorType string

const (
	ActorTypeUser   ActorType = "user"
	ActorTypeSystem ActorType = "system"
	ActorTypeAgent  ActorType = "agent"
)

// ActorRef points at a human or automated actor.
type ActorRef struct {
	Type ActorType `json:"type"`
	ID   string    `json:"id"`
}

// RiskLevel is the coarse risk bucket exposed by the core.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// RiskDecision is the graded recommendation produced by the risk evaluator.
type RiskDecision string

const (
	RiskDecisionAllow           RiskDecision = "allow"
	RiskDecisionEnhancedLog     RiskDecision = "enhanced_log"
	RiskDecisionAdditionalAuth  RiskDecision = "additional_auth"
	RiskDecisionRequireApproval RiskDecision = "require_approval"
	RiskDecisionDeny            RiskDecision = "deny"
)

// EvidenceQuality describes how trustworthy a factor input was.
type EvidenceQuality string

const (
	EvidenceQualityObserved EvidenceQuality = "observed"
	EvidenceQualityInferred EvidenceQuality = "inferred"
	EvidenceQualityMissing  EvidenceQuality = "missing"
	EvidenceQualityStale    EvidenceQuality = "stale"
)

// RiskFactorType separates pro-risk and anti-risk contributions.
type RiskFactorType string

const (
	RiskFactorTypePro  RiskFactorType = "pro"
	RiskFactorTypeAnti RiskFactorType = "anti"
)

// RiskProfileRef identifies the immutable risk profile used for evaluation.
type RiskProfileRef struct {
	Name    string
	Version int
}

// EvidenceStatus summarizes whether a deterministic check passed.
type EvidenceStatus string

const (
	EvidenceStatusPassed  EvidenceStatus = "passed"
	EvidenceStatusFailed  EvidenceStatus = "failed"
	EvidenceStatusSkipped EvidenceStatus = "skipped"
)

// ApprovalStatus tracks manual approval state.
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
)

// LeaseStatus tracks execution lease lifecycle.
type LeaseStatus string

const (
	LeaseStatusActive  LeaseStatus = "active"
	LeaseStatusUsed    LeaseStatus = "used"
	LeaseStatusExpired LeaseStatus = "expired"
)

// Action is the aggregate root for protected financial operations.
type Action struct {
	ID            uuid.UUID
	Type          ActionType
	Status        ActionStatus
	Decision      Decision
	ResourceID    string
	ResourceType  ResourceType
	SourceSystem  string
	Justification string
	RequestedBy   ActorRef
	ProposedBy    ActorRef
	Payload       json.RawMessage
	Metadata      map[string]any
	Risk          RiskAssessment
	Evidence      []EvidenceRecord
	Approval      *Approval
	Lease         *ExecutionLease
	Execution     *ExecutionResult
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// RiskAssessment is the deterministic risk output attached to an action.
type RiskAssessment struct {
	Level               RiskLevel
	Score               int
	Summary             string
	Profile             RiskProfileRef
	RiskPressure        float64
	SafetyPressure      float64
	RawScore            float64
	DecisionScore       float64
	RecommendedDecision RiskDecision
	Factors             []RiskFactor
	Amplifications      []RiskInteraction
	Attenuations        []RiskInteraction
}

// RiskFactor explains why the risk landed where it did.
type RiskFactor struct {
	Code            string
	Type            RiskFactorType
	Active          bool
	Weight          float64
	AppliedWeight   float64
	Summary         string
	EvidenceQuality EvidenceQuality
}

// RiskInteraction explains a multi-factor amplification or attenuation.
type RiskInteraction struct {
	Factors    []string
	Multiplier float64
	Summary    string
}

// EvidenceRecord stores one deterministic validation or evidence item.
type EvidenceRecord struct {
	ID        uuid.UUID
	ActionID  uuid.UUID
	Kind      string
	Status    EvidenceStatus
	Summary   string
	Details   map[string]any
	CreatedAt time.Time
}

// Approval captures the approval state attached to an action.
type Approval struct {
	ID            uuid.UUID
	ActionID      uuid.UUID
	Status        ApprovalStatus
	RequiredCount int
	GrantedCount  int
	DecidedBy     *ActorRef
	Comment       string
	ExpiresAt     time.Time
	DecidedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ExecutionLease authorizes one bounded execution of an action.
type ExecutionLease struct {
	ID        uuid.UUID
	ActionID  uuid.UUID
	Status    LeaseStatus
	Scope     LeaseScope
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// LeaseScope defines what the lease can be used for.
type LeaseScope struct {
	ActionID     uuid.UUID
	ActionType   ActionType
	ResourceID   string
	ResourceType ResourceType
}

// ExecutionResult stores the terminal execution summary.
type ExecutionResult struct {
	Status     string
	ExecutedBy ActorRef
	Result     map[string]any
	ExecutedAt time.Time
}

// ProtectedResource is the domain object an action operates on.
type ProtectedResource struct {
	ID          string
	Type        ResourceType
	Name        string
	Environment string
	Chain       string
	Labels      map[string]string
	Criticality string
}

// WithdrawalPayload is the payload shape for withdrawal actions.
type WithdrawalPayload struct {
	Asset              string `json:"asset"`
	Amount             string `json:"amount"`
	Network            string `json:"network"`
	DestinationAddress string `json:"destination_address"`
}

// TreasuryTransferPayload is the payload shape for treasury transfer actions.
type TreasuryTransferPayload struct {
	Asset       string `json:"asset"`
	Amount      string `json:"amount"`
	FromAccount string `json:"from_account"`
	ToAccount   string `json:"to_account"`
}

// HotToColdMovePayload is the payload shape for hot to cold wallet moves.
type HotToColdMovePayload struct {
	Asset      string `json:"asset"`
	Amount     string `json:"amount"`
	Network    string `json:"network"`
	FromWallet string `json:"from_wallet"`
	ToWallet   string `json:"to_wallet"`
}
