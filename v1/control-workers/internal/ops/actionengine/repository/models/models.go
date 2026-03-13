package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type CatalogItem struct {
	ActionType       string         `gorm:"column:action_type;primaryKey"`
	SchemaJSON       datatypes.JSON `gorm:"column:schema_json"`
	RequiresApproval bool           `gorm:"column:requires_approval"`
	MaxTTLSeconds    int            `gorm:"column:max_ttl_seconds"`
	Enabled          bool           `gorm:"column:enabled"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
}

func (CatalogItem) TableName() string { return "ops_action_catalog" }

type Proposal struct {
	ID               uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	OrgID            uuid.UUID      `gorm:"column:org_id;type:uuid"`
	IncidentID       *uuid.UUID     `gorm:"column:incident_id;type:uuid"`
	ActionType       string         `gorm:"column:action_type"`
	ScopeJSON        datatypes.JSON `gorm:"column:scope_json"`
	ParamsJSON       datatypes.JSON `gorm:"column:params_json"`
	TTLSeconds       int            `gorm:"column:ttl_seconds"`
	EvidenceRefsJSON datatypes.JSON `gorm:"column:evidence_refs_json"`
	IdempotencyKey   string         `gorm:"column:idempotency_key"`
	Status           string         `gorm:"column:status"`
	ApprovalRequired bool           `gorm:"column:approval_required"`
	ProposedBy       *string        `gorm:"column:proposed_by"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
}

func (Proposal) TableName() string { return "ops_action_proposals" }

type Execution struct {
	ID           uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	ProposalID   uuid.UUID      `gorm:"column:proposal_id;type:uuid"`
	OrgID        uuid.UUID      `gorm:"column:org_id;type:uuid"`
	Mode         string         `gorm:"column:mode"`
	Status       string         `gorm:"column:status"`
	ErrorCode    *string        `gorm:"column:error_code"`
	ErrorMessage *string        `gorm:"column:error_message"`
	OutputJSON   datatypes.JSON `gorm:"column:output_json"`
	ExecutedBy   *string        `gorm:"column:executed_by"`
	ExecutedAt   time.Time      `gorm:"column:executed_at"`
}

func (Execution) TableName() string { return "ops_action_executions" }

type Approval struct {
	ID         uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	ProposalID uuid.UUID `gorm:"column:proposal_id;type:uuid"`
	OrgID      uuid.UUID `gorm:"column:org_id;type:uuid"`
	Approved   bool      `gorm:"column:approved"`
	Approver   string    `gorm:"column:approver"`
	Comment    *string   `gorm:"column:comment"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (Approval) TableName() string { return "ops_action_approvals" }
