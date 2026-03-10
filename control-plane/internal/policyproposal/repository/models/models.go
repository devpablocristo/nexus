package models

import (
	"time"

	"github.com/google/uuid"
)

type Proposal struct {
	ID             uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	OrgID          uuid.UUID  `gorm:"column:org_id;type:uuid;not null"`
	Status         string     `gorm:"column:status;not null"`
	Diff           []byte     `gorm:"column:diff_json;type:jsonb;not null"`
	Rationale      string     `gorm:"column:rationale;not null"`
	TestsSuggested []byte     `gorm:"column:tests_suggested_json;type:jsonb;not null"`
	RollbackPlan   string     `gorm:"column:rollback_plan;not null"`
	CreatedBy      *string    `gorm:"column:created_by"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	DecidedBy      *string    `gorm:"column:decided_by"`
	DecidedAt      *time.Time `gorm:"column:decided_at"`
}

func (Proposal) TableName() string { return "policy_proposals" }

type PolicyVersion struct {
	ID         uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	OrgID      uuid.UUID  `gorm:"column:org_id;type:uuid;not null"`
	ProposalID *uuid.UUID `gorm:"column:proposal_id;type:uuid"`
	Label      string     `gorm:"column:version_label;not null"`
	Spec       []byte     `gorm:"column:spec_json;type:jsonb;not null"`
	Mode       string     `gorm:"column:mode;not null"`
	CreatedBy  *string    `gorm:"column:created_by"`
	CreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (PolicyVersion) TableName() string { return "policy_versions" }
