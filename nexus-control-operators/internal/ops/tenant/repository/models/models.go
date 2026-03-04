package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type TenantProfile struct {
	OrgID               uuid.UUID      `gorm:"column:org_id;type:uuid;primaryKey"`
	Tier                string         `gorm:"column:tier"`
	MaxTTLSeconds       int            `gorm:"column:max_ttl_seconds"`
	AutoMitigateEnabled bool           `gorm:"column:auto_mitigate_enabled"`
	CostModelJSON       datatypes.JSON `gorm:"column:cost_model_json"`
	CreatedAt           time.Time      `gorm:"column:created_at"`
	UpdatedAt           time.Time      `gorm:"column:updated_at"`
}

func (TenantProfile) TableName() string { return "ops_tenant_registry" }

type Contact struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"column:org_id;type:uuid"`
	Name        string    `gorm:"column:name"`
	Channel     string    `gorm:"column:channel"`
	Destination string    `gorm:"column:destination"`
	SeverityMin string    `gorm:"column:severity_min"`
	IsPrimary   bool      `gorm:"column:is_primary"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (Contact) TableName() string { return "ops_tenant_contacts" }

type IncidentSettings struct {
	OrgID                         uuid.UUID      `gorm:"column:org_id;type:uuid;primaryKey"`
	AutoOpenThresholdJSON         datatypes.JSON `gorm:"column:auto_open_threshold_json"`
	CooldownSeconds               int            `gorm:"column:cooldown_seconds"`
	MonitoringWindowSeconds       int            `gorm:"column:monitoring_window_seconds"`
	ExternalCommsRequiresApproval bool           `gorm:"column:external_comms_requires_approval"`
	UpdatedAt                     time.Time      `gorm:"column:updated_at"`
}

func (IncidentSettings) TableName() string { return "ops_incident_settings" }
