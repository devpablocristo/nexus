package domain

import (
	"time"

	"github.com/google/uuid"
)

// Effect is the policy outcome applied when the policy matches.
type Effect string

const (
	// EffectAllow allows execution to continue.
	EffectAllow Effect = "allow"
	// EffectDeny blocks execution before the upstream call.
	EffectDeny Effect = "deny"
)

// Policy is the minimal v2 policy model used by /run.
type Policy struct {
	ID                 uuid.UUID
	ToolName           string
	Effect             Effect
	Priority           int
	Expression         string
	Reason             string
	RequireApproval    bool
	ApprovalTTLSeconds int
	Enabled            bool
	Archived           bool
	ArchivedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
