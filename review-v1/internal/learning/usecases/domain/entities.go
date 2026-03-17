package domain

import (
	"time"

	"github.com/google/uuid"
)

type PolicyProposal struct {
	ID                 uuid.UUID
	ProposedName       string
	ProposedDescription string
	ProposedExpression string
	ProposedEffect     string
	ProposedActionType *string
	ProposedPriority   int
	PatternSummary    string
	Confidence        float64
	SampleSize        int
	TimeWindow        string
	Status            string
	DecidedBy         *string
	DecidedAt         *time.Time
	PolicyID          *uuid.UUID
	CreatedAt         time.Time
}

const (
	ProposalStatusPending   = "pending"
	ProposalStatusAccepted  = "accepted"
	ProposalStatusDismissed = "dismissed"
	ProposalStatusExpired   = "expired"
)
