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

type ProtectedResourceItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ResourceType string  `json:"resource_type"`
	MatchValue   string  `json:"match_value"`
	MatchMode    string  `json:"match_mode"`
	Environment  string  `json:"environment"`
	Reason       string  `json:"reason,omitempty"`
	Enabled      bool    `json:"enabled"`
	CreatedBy    *string `json:"created_by,omitempty"`
	UpdatedBy    *string `json:"updated_by,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type ProtectedResourcesResponse struct {
	Items []ProtectedResourceItem `json:"items"`
}

type CreateProtectedResourceRequest struct {
	Name         string `json:"name" binding:"required"`
	ResourceType string `json:"resource_type" binding:"required"`
	MatchValue   string `json:"match_value" binding:"required"`
	MatchMode    string `json:"match_mode"`
	Environment  string `json:"environment"`
	Reason       string `json:"reason"`
	Enabled      *bool  `json:"enabled"`
}

type RestoreEvidenceItem struct {
	ID             string         `json:"id"`
	Environment    string         `json:"environment"`
	System         string         `json:"system"`
	Status         string         `json:"status"`
	SnapshotID     string         `json:"snapshot_id,omitempty"`
	RestoreTarget  string         `json:"restore_target,omitempty"`
	StartedAt      string         `json:"started_at,omitempty"`
	CompletedAt    string         `json:"completed_at,omitempty"`
	Source         string         `json:"source,omitempty"`
	ArtifactSHA256 string         `json:"artifact_sha256,omitempty"`
	Summary        map[string]any `json:"summary"`
	CreatedAt      string         `json:"created_at"`
}

type RestoreEvidenceResponse struct {
	Items []RestoreEvidenceItem `json:"items"`
}

type RecordRestoreEvidenceRequest struct {
	Environment    string         `json:"environment"`
	System         string         `json:"system"`
	Status         string         `json:"status"`
	SnapshotID     string         `json:"snapshot_id"`
	RestoreTarget  string         `json:"restore_target"`
	StartedAt      string         `json:"started_at"`
	CompletedAt    string         `json:"completed_at"`
	Source         string         `json:"source"`
	ArtifactSHA256 string         `json:"artifact_sha256"`
	Summary        map[string]any `json:"summary"`
}
