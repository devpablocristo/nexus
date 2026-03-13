package dto

import "time"

type CreatePolicyRequest struct {
	ToolName           string `json:"tool_name"`
	Effect             string `json:"effect"`
	Priority           int    `json:"priority"`
	Expression         string `json:"expression"`
	Reason             string `json:"reason"`
	RequireApproval    bool   `json:"require_approval"`
	ApprovalTTLSeconds int    `json:"approval_ttl_seconds"`
	Enabled            *bool  `json:"enabled"`
}

type UpdatePolicyRequest struct {
	ToolName           *string `json:"tool_name"`
	Effect             *string `json:"effect"`
	Priority           *int    `json:"priority"`
	Expression         *string `json:"expression"`
	Reason             *string `json:"reason"`
	RequireApproval    *bool   `json:"require_approval"`
	ApprovalTTLSeconds *int    `json:"approval_ttl_seconds"`
	Enabled            *bool   `json:"enabled"`
}

type PolicyResponse struct {
	ID                 string     `json:"id"`
	ToolName           string     `json:"tool_name"`
	Effect             string     `json:"effect"`
	Priority           int        `json:"priority"`
	Expression         string     `json:"expression"`
	Reason             string     `json:"reason"`
	RequireApproval    bool       `json:"require_approval"`
	ApprovalTTLSeconds int        `json:"approval_ttl_seconds"`
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
