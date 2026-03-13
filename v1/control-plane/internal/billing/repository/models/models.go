package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type TenantSettings struct {
	OrgID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	PlanCode             string
	HardLimits           datatypes.JSON `gorm:"column:hard_limits_json;type:jsonb"`
	StripeCustomerID     *string
	StripeSubscriptionID *string
	BillingStatus        string
	PastDueSince         *time.Time
	UpdatedBy            *string
	UpdatedAt            time.Time
	CreatedAt            time.Time
}

func (TenantSettings) TableName() string { return "tenant_settings" }

type Org struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string
}

func (Org) TableName() string { return "orgs" }
