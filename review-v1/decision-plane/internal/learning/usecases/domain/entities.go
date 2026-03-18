package domain

import (
	"time"

	"github.com/google/uuid"
)

type PolicyProposal struct {
	ID                  uuid.UUID  `json:"id"`
	ProposedName        string     `json:"proposed_name"`
	ProposedDescription string     `json:"proposed_description,omitempty"`
	ProposedExpression  string     `json:"proposed_expression"`
	ProposedEffect      string     `json:"proposed_effect"`
	ProposedActionType  *string    `json:"proposed_action_type,omitempty"`
	ProposedPriority    int        `json:"proposed_priority"`
	PatternSummary      string     `json:"pattern_summary"`
	Confidence          float64    `json:"confidence"`
	SampleSize          int        `json:"sample_size"`
	TimeWindow          string     `json:"time_window"`
	Status              string     `json:"status"`
	DecidedBy           *string    `json:"decided_by,omitempty"`
	DecidedAt           *time.Time `json:"decided_at,omitempty"`
	PolicyID            *uuid.UUID `json:"policy_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

const (
	ProposalStatusPending   = "pending"
	ProposalStatusAccepted  = "accepted"
	ProposalStatusDismissed = "dismissed"
	ProposalStatusExpired   = "expired"
)
