package dto

type CreatePolicyRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	ActionType   *string `json:"action_type,omitempty"`
	TargetSystem *string `json:"target_system,omitempty"`
	Expression   string  `json:"expression"`
	Effect       string  `json:"effect"`
	RiskOverride *string `json:"risk_override,omitempty"`
	Priority     int     `json:"priority"`
	Enabled      bool    `json:"enabled"`
}

type UpdatePolicyRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	ActionType   *string `json:"action_type,omitempty"`
	TargetSystem *string `json:"target_system,omitempty"`
	Expression   *string `json:"expression,omitempty"`
	Effect       *string `json:"effect,omitempty"`
	RiskOverride *string `json:"risk_override,omitempty"`
	Priority     *int    `json:"priority,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

type PolicyResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	ActionType   *string `json:"action_type,omitempty"`
	TargetSystem *string `json:"target_system,omitempty"`
	Expression   string  `json:"expression"`
	Effect       string  `json:"effect"`
	RiskOverride *string `json:"risk_override,omitempty"`
	Priority     int     `json:"priority"`
	Origin       string  `json:"origin"`
	Enabled      bool    `json:"enabled"`
	ArchivedAt   *string `json:"archived_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}
