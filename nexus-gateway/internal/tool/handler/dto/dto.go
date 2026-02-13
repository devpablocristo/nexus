package dto

import "time"

type ToolResponse struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Description  *string        `json:"description,omitempty"`
	Method       string         `json:"method"`
	URL          string         `json:"url"`
	InputSchema  map[string]any `json:"input_schema"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	ActionType   string         `json:"action_type"`
	RiskLevel    int            `json:"risk_level"`
	Enabled      bool           `json:"enabled"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type ListToolsResponse struct {
	Items []ToolResponse `json:"items"`
}
