package domain

import (
	"time"

	"github.com/google/uuid"
)

type Metric string

const (
	MetricDenyRate       Metric = "deny_rate"
	MetricErrorRate      Metric = "error_rate"
	MetricLatencyP95     Metric = "latency_p95"
	MetricRateLimitCount Metric = "rate_limited_count"
)

type AlertRule struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	Name            string
	Metric          Metric
	Threshold       float64
	WindowSeconds   int
	ToolName        *string
	WebhookURL      string
	CooldownSeconds int
	Enabled         bool
	LastFiredAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WebhookPayload struct {
	AlertRuleID   string  `json:"alert_rule_id"`
	AlertName     string  `json:"alert_name"`
	Metric        string  `json:"metric"`
	Threshold     float64 `json:"threshold"`
	CurrentValue  float64 `json:"current_value"`
	ToolName      string  `json:"tool_name,omitempty"`
	WindowSeconds int     `json:"window_seconds"`
	FiredAt       string  `json:"fired_at"`
}
