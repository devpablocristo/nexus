package domain

import (
	"time"

	"github.com/google/uuid"
)

type Severity string

type Status string

const (
	SeverityLow  Severity = "LOW"
	SeverityMed  Severity = "MED"
	SeverityHigh Severity = "HIGH"
	SeverityCrit Severity = "CRIT"

	StatusOpen   Status = "open"
	StatusClosed Status = "closed"
)

type Incident struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	Severity         Severity
	Status           Status
	Title            string
	Summary          string
	RelatedActionIDs []string
	EvidenceRefs     []string
	CreatedBy        *string
	OpenedAt         time.Time
	ClosedAt         *time.Time
}
