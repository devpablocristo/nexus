package dto

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

type UsageSummaryResponse struct {
	Period   string        `json:"period"`
	Counters UsageCounters `json:"counters"`
}

type BillingStatusResponse struct {
	PlanCode         string               `json:"plan_code"`
	BillingStatus    string               `json:"billing_status"`
	CurrentPeriodEnd *string              `json:"current_period_end,omitempty"`
	HardLimits       HardLimits           `json:"hard_limits"`
	Usage            UsageSummaryResponse `json:"usage"`
}

type CheckoutRequest struct {
	PlanCode   string `json:"plan_code" binding:"required"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type CheckoutResponse struct {
	URL string `json:"url"`
}

type PortalRequest struct {
	ReturnURL string `json:"return_url"`
}

type PortalResponse struct {
	URL string `json:"url"`
}
