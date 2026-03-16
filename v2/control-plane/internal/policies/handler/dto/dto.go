package dto

import "time"

type CreatePolicyRequest struct {
	ActionType         string `json:"action_type"`
	ResourceType       string `json:"resource_type"`
	Effect             string `json:"effect"`
	Priority           int    `json:"priority"`
	Expression         string `json:"expression"`
	Reason             string `json:"reason"`
	RequireApproval    bool   `json:"require_approval"`
	ApprovalTTLSeconds int    `json:"approval_ttl_seconds"`
	IsTrap             bool   `json:"is_trap"`
	Enabled            *bool  `json:"enabled"`
}

type UpdatePolicyRequest struct {
	ActionType         *string `json:"action_type,omitempty"`
	ResourceType       *string `json:"resource_type,omitempty"`
	Effect             *string `json:"effect,omitempty"`
	Priority           *int    `json:"priority,omitempty"`
	Expression         *string `json:"expression,omitempty"`
	Reason             *string `json:"reason,omitempty"`
	RequireApproval    *bool   `json:"require_approval,omitempty"`
	ApprovalTTLSeconds *int    `json:"approval_ttl_seconds,omitempty"`
	IsTrap             *bool   `json:"is_trap,omitempty"`
	Enabled            *bool   `json:"enabled,omitempty"`
}

type PolicyResponse struct {
	ID                 string     `json:"id"`
	ActionType         string     `json:"action_type"`
	ResourceType       string     `json:"resource_type"`
	Effect             string     `json:"effect"`
	Priority           int        `json:"priority"`
	Expression         string     `json:"expression"`
	Reason             string     `json:"reason"`
	RequireApproval    bool       `json:"require_approval"`
	ApprovalTTLSeconds int        `json:"approval_ttl_seconds"`
	IsTrap             bool       `json:"is_trap"`
	Enabled            bool       `json:"enabled"`
	Archived           bool       `json:"archived"`
	ArchivedAt         *time.Time `json:"archived_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type ListPoliciesResponse struct {
	Items []PolicyResponse `json:"items"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
