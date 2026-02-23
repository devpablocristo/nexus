package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Draft struct {
	ID               uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	OrgID            uuid.UUID      `gorm:"column:org_id;type:uuid"`
	IncidentID       *uuid.UUID     `gorm:"column:incident_id;type:uuid"`
	Channel          string         `gorm:"column:channel"`
	Audience         string         `gorm:"column:audience"`
	Status           string         `gorm:"column:status"`
	ContentJSON      datatypes.JSON `gorm:"column:content_json"`
	RequiresApproval bool           `gorm:"column:requires_approval"`
	CreatedBy        *string        `gorm:"column:created_by"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	SentAt           *time.Time     `gorm:"column:sent_at"`
}

func (Draft) TableName() string { return "ops_comms_drafts" }
