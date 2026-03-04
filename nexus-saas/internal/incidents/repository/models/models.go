package models

import (
	"time"

	"github.com/google/uuid"
)

type Incident struct {
	ID               uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	OrgID            uuid.UUID  `gorm:"column:org_id;type:uuid;not null"`
	Severity         string     `gorm:"column:severity;not null"`
	Status           string     `gorm:"column:status;not null"`
	Title            string     `gorm:"column:title;not null"`
	Summary          string     `gorm:"column:summary;not null"`
	RelatedActionIDs []byte     `gorm:"column:related_action_ids_json;type:jsonb;not null"`
	EvidenceRefs     []byte     `gorm:"column:evidence_refs_json;type:jsonb;not null"`
	CreatedBy        *string    `gorm:"column:created_by"`
	OpenedAt         time.Time  `gorm:"column:opened_at;autoCreateTime"`
	ClosedAt         *time.Time `gorm:"column:closed_at"`
}

func (Incident) TableName() string { return "incidents" }
