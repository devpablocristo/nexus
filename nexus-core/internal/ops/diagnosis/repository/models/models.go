package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Report struct {
	ID              uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	OrgID           uuid.UUID      `gorm:"column:org_id;type:uuid"`
	IncidentID      *uuid.UUID     `gorm:"column:incident_id;type:uuid"`
	Provider        string         `gorm:"column:provider"`
	Model           string         `gorm:"column:model"`
	Status          string         `gorm:"column:status"`
	ReportJSON      datatypes.JSON `gorm:"column:report_json"`
	ValidationError *string        `gorm:"column:validation_error"`
	CreatedBy       *string        `gorm:"column:created_by"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
}

func (Report) TableName() string { return "ops_diagnosis_reports" }
