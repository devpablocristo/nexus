package dto

type ApprovalDecisionRequest struct {
	DecidedBy string `json:"decided_by"`
	Note      string `json:"note,omitempty"`
}
