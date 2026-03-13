package dto

import "time"

type PolicyResponse struct {
	ID             string         `json:"id"`
	ToolID         string         `json:"tool_id"`
	Effect         string         `json:"effect"`
	Priority       int            `json:"priority"`
	Conditions     map[string]any `json:"conditions"`
	Limits         map[string]any `json:"limits"`
	ReasonTemplate string         `json:"reason_template"`
	Enabled        bool           `json:"enabled"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type ListPoliciesResponse struct {
	Items []PolicyResponse `json:"items"`
}
