package domain

import (
	"time"

	"github.com/google/uuid"
)

type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

type Policy struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	ToolID         uuid.UUID
	Effect         Effect
	Priority       int
	ConditionsJSON []byte
	LimitsJSON     []byte
	ReasonTemplate string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
