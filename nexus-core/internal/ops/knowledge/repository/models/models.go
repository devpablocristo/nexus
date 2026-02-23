package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Document struct {
	ID        uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	OrgID     uuid.UUID      `gorm:"column:org_id;type:uuid"`
	DocType   string         `gorm:"column:doc_type"`
	Title     string         `gorm:"column:title"`
	BodyMD    string         `gorm:"column:body_md"`
	TagsJSON  datatypes.JSON `gorm:"column:tags_json"`
	SourceRef *string        `gorm:"column:source_ref"`
	CreatedBy *string        `gorm:"column:created_by"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
}

func (Document) TableName() string { return "ops_knowledge_docs" }
