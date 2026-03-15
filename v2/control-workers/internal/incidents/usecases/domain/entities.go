package domain

import "time"

type SourceKind string

const (
	SourceKindAction SourceKind = "action"
)

type Trigger string

const (
	TriggerBlockedAction    Trigger = "blocked_action"
	TriggerApprovalRejected Trigger = "approval_rejected"
	TriggerApprovalExpired  Trigger = "approval_expired"
	TriggerExecutionFailed  Trigger = "execution_failed"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Status string

const (
	StatusOpen         Status = "open"
	StatusAcknowledged Status = "acknowledged"
	StatusResolved     Status = "resolved"
)

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

type Incident struct {
	ID           string
	SourceKind   SourceKind
	SourceID     string
	ActionType   string
	ResourceID   string
	ResourceType string
	Trigger      Trigger
	RiskLevel    RiskLevel
	Severity     Severity
	Status       Status
	Summary      string
	Reason       string
	Details      map[string]any
	ArchivedAt   *time.Time
	ResolvedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
