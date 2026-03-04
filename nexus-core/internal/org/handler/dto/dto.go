package dto

import "github.com/google/uuid"

type CreateOrgRequest struct {
	Name   string   `json:"name" binding:"required"`
	Scopes []string `json:"scopes"`
}

type CreateOrgResponse struct {
	OrgID  uuid.UUID `json:"org_id"`
	APIKey string    `json:"api_key"`
	Name   string    `json:"name"`
}
