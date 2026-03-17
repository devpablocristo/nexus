package domain

import (
	"time"

	"github.com/google/uuid"
)

type Policy struct {
	ID          uuid.UUID
	Name        string
	Description string
	ActionType  *string
	TargetSystem *string
	Expression  string
	Effect      string
	RiskOverride *string
	Priority    int
	Origin      string
	ProposalID  *uuid.UUID
	Enabled     bool
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
