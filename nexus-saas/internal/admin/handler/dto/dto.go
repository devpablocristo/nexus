package dto

type BootstrapResponse struct {
	OrgID         string         `json:"org_id"`
	Actor         *string        `json:"actor,omitempty"`
	Role          *string        `json:"role,omitempty"`
	Scopes        []string       `json:"scopes"`
	AuthMethod    string         `json:"auth_method"`
	CanReadAdmin  bool           `json:"can_read_admin"`
	CanWriteAdmin bool           `json:"can_write_admin"`
	TenantSetting TenantSettings `json:"tenant_settings"`
}

type TenantSettings struct {
	PlanCode   string         `json:"plan_code"`
	Status     string         `json:"status"`
	DeletedAt  string         `json:"deleted_at,omitempty"`
	HardLimits map[string]any `json:"hard_limits"`
	UpdatedBy  *string        `json:"updated_by,omitempty"`
	UpdatedAt  string         `json:"updated_at,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
}

type UpsertTenantSettingsRequest struct {
	PlanCode   string         `json:"plan_code" binding:"required"`
	HardLimits map[string]any `json:"hard_limits"`
}

type AdminActivityItem struct {
	ID           string         `json:"id"`
	Actor        *string        `json:"actor,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   *string        `json:"resource_id,omitempty"`
	Payload      map[string]any `json:"payload"`
	CreatedAt    string         `json:"created_at"`
}

type AdminActivityResponse struct {
	Items []AdminActivityItem `json:"items"`
}
