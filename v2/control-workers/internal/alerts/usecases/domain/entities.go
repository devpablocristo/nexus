package domain

import "time"

type SourceKind string

const (
	SourceKindIncident SourceKind = "incident"
)

type Channel string

const (
	ChannelSlack     Channel = "slack"
	ChannelPagerDuty Channel = "pagerduty"
	ChannelEmail     Channel = "email"
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
	StatusPending      Status = "pending"
	StatusDispatched   Status = "dispatched"
	StatusSuppressed   Status = "suppressed"
	StatusAcknowledged Status = "acknowledged"
)

type Alert struct {
	ID         string
	SourceKind SourceKind
	SourceID   string
	Channel    Channel
	Route      string
	Severity   Severity
	Status     Status
	Summary    string
	Body       string
	Details    map[string]any
	ArchivedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
