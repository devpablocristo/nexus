package models

import (
	"time"

	"github.com/google/uuid"
)

type Action struct {
	ID           uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	OrgID        uuid.UUID  `gorm:"column:org_id;type:uuid;not null"`
	ScopeType    string     `gorm:"column:scope_type;not null"`
	ScopeID      *string    `gorm:"column:scope_id"`
	ActionType   string     `gorm:"column:action_type;not null"`
	Params       []byte     `gorm:"column:params_json;type:jsonb;not null"`
	TTLSeconds   int        `gorm:"column:ttl_seconds;not null"`
	Status       string     `gorm:"column:status;not null"`
	EvidenceRefs []byte     `gorm:"column:evidence_refs_json;type:jsonb;not null"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime"`
	CreatedBy    *string    `gorm:"column:created_by"`
	RolledBackAt *time.Time `gorm:"column:rolled_back_at"`
	RolledBackBy *string    `gorm:"column:rolled_back_by"`
}

func (Action) TableName() string { return "actions" }
