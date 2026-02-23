package domain

import (
	"time"

	"github.com/google/uuid"
)

type Tier string

const (
	TierStarter    Tier = "starter"
	TierGrowth     Tier = "growth"
	TierEnterprise Tier = "enterprise"
)

type TenantProfile struct {
	OrgID               uuid.UUID
	Tier                Tier
	MaxTTLSeconds       int
	AutoMitigateEnabled bool
	CostModel           map[string]any
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Contact struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Name        string
	Channel     string
	Destination string
	SeverityMin string
	IsPrimary   bool
	CreatedAt   time.Time
}

type IncidentSettings struct {
	OrgID                          uuid.UUID
	AutoOpenThreshold              map[string]any
	CooldownSeconds                int
	MonitoringWindowSeconds        int
	ExternalCommsRequiresApproval  bool
	UpdatedAt                      time.Time
}
