package dto

import "time"

type AlertRuleItem struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Metric          string     `json:"metric"`
	Threshold       float64    `json:"threshold"`
	WindowSeconds   int        `json:"window_seconds"`
	ToolName        *string    `json:"tool_name,omitempty"`
	WebhookURL      string     `json:"webhook_url"`
	CooldownSeconds int        `json:"cooldown_seconds"`
	Enabled         bool       `json:"enabled"`
	LastFiredAt     *time.Time `json:"last_fired_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type ListAlertRulesResponse struct {
	Items []AlertRuleItem `json:"items"`
}

type CreateAlertRuleRequest struct {
	Name            string  `json:"name" binding:"required"`
	Metric          string  `json:"metric" binding:"required"`
	Threshold       float64 `json:"threshold"`
	WindowSeconds   int     `json:"window_seconds"`
	ToolName        *string `json:"tool_name,omitempty"`
	WebhookURL      string  `json:"webhook_url" binding:"required"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	Enabled         bool    `json:"enabled"`
}
