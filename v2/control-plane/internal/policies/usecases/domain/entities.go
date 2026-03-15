package domain

import "time"

// Effect is the action policy outcome applied when the policy matches.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Policy is the control-plane policy model for protected actions.
type Policy struct {
	ID                 string
	ActionType         string
	ResourceType       string
	Effect             Effect
	Priority           int
	Expression         string
	Reason             string
	RequireApproval    bool
	ApprovalTTLSeconds int
	Enabled            bool
	ArchivedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
