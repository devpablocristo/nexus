package domain

import (
	"time"

	"github.com/google/uuid"
)

type PlanCode string

const (
	PlanStarter    PlanCode = "starter"
	PlanGrowth     PlanCode = "growth"
	PlanEnterprise PlanCode = "enterprise"
)

type BillingStatus string

const (
	BillingTrialing BillingStatus = "trialing"
	BillingActive   BillingStatus = "active"
	BillingPastDue  BillingStatus = "past_due"
	BillingCanceled BillingStatus = "canceled"
	BillingUnpaid   BillingStatus = "unpaid"
)

type HardLimits struct {
	ToolsMax           int `json:"tools_max"`
	RunRPM             int `json:"run_rpm"`
	AuditRetentionDays int `json:"audit_retention_days"`
}

type UsageCounters struct {
	APICalls        int64 `json:"api_calls"`
	EventsIngested  int64 `json:"events_ingested"`
	IncidentsOpened int64 `json:"incidents_opened"`
	ActionsExecuted int64 `json:"actions_executed"`
}

type UsageSummary struct {
	Period   string        `json:"period"`
	Counters UsageCounters `json:"counters"`
}

type TenantBilling struct {
	OrgID                uuid.UUID
	PlanCode             PlanCode
	HardLimits           HardLimits
	BillingStatus        BillingStatus
	PastDueSince         *time.Time
	StripeCustomerID     *string
	StripeSubscriptionID *string
	UpdatedAt            time.Time
	CreatedAt            time.Time
}

type BillingStatusView struct {
	PlanCode         PlanCode      `json:"plan_code"`
	BillingStatus    BillingStatus `json:"billing_status"`
	CurrentPeriodEnd *time.Time    `json:"current_period_end,omitempty"`
	HardLimits       HardLimits    `json:"hard_limits"`
	Usage            UsageSummary  `json:"usage"`
}

type CheckoutInput struct {
	OrgID         uuid.UUID
	PlanCode      PlanCode
	SuccessURL    string
	CancelURL     string
	Actor         *string
	CustomerEMail *string
}

type PortalInput struct {
	OrgID     uuid.UUID
	ReturnURL string
}
