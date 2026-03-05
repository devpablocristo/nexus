package dto

type UserInfo struct {
	ID         string  `json:"id"`
	ExternalID string  `json:"external_id"`
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	AvatarURL  *string `json:"avatar_url,omitempty"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type MeResponse struct {
	OrgID      string    `json:"org_id"`
	ExternalID string    `json:"external_id"`
	Role       string    `json:"role,omitempty"`
	Scopes     []string  `json:"scopes,omitempty"`
	User       *UserInfo `json:"user,omitempty"`
}

type OrgMemberItem struct {
	ID       string   `json:"id"`
	OrgID    string   `json:"org_id"`
	UserID   string   `json:"user_id"`
	Role     string   `json:"role"`
	JoinedAt string   `json:"joined_at"`
	User     UserInfo `json:"user"`
}

type ListOrgMembersResponse struct {
	Items []OrgMemberItem `json:"items"`
}

type APIKeyItem struct {
	ID        string   `json:"id"`
	OrgID     string   `json:"org_id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"created_at"`
}

type ListAPIKeysResponse struct {
	Items []APIKeyItem `json:"items"`
}

type CreateAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type CreateAPIKeyResponse struct {
	ID        string   `json:"id"`
	OrgID     string   `json:"org_id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	APIKey    string   `json:"api_key"`
	CreatedAt string   `json:"created_at"`
}

type RotateAPIKeyResponse struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	APIKey    string `json:"api_key"`
	RotatedAt string `json:"rotated_at"`
}
