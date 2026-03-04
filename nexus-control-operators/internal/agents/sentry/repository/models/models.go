package models

import (
	"github.com/google/uuid"
)

// BaselineRow modelo de persistencia para ops_sentry_baselines.
type BaselineRow struct {
	OrgID       uuid.UUID `gorm:"column:org_id;type:uuid;primaryKey"`
	ToolName    string    `gorm:"column:tool_name;primaryKey"`
	Metric      string    `gorm:"column:metric;primaryKey"`
	EWMAValue   float64   `gorm:"column:ewma_value"`
	SampleCount int64     `gorm:"column:sample_count"`
}

func (BaselineRow) TableName() string { return "ops_sentry_baselines" }

// FingerprintRow modelo de persistencia para ops_incident_fingerprints.
type FingerprintRow struct {
	OrgID       uuid.UUID  `gorm:"column:org_id;type:uuid;primaryKey"`
	Fingerprint string     `gorm:"column:fingerprint;primaryKey"`
	IncidentID  *uuid.UUID `gorm:"column:incident_id;type:uuid"`
	State       string     `gorm:"column:state"`
}

func (FingerprintRow) TableName() string { return "ops_incident_fingerprints" }
