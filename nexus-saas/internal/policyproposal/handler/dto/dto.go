package dto

type CreateProposalRequest struct {
	Status         string         `json:"status"`
	Diff           map[string]any `json:"diff"`
	Rationale      string         `json:"rationale"`
	TestsSuggested []string       `json:"tests_suggested"`
	RollbackPlan   string         `json:"rollback_plan"`
}

type ProposalItem struct {
	ID             string         `json:"id"`
	Status         string         `json:"status"`
	Diff           map[string]any `json:"diff"`
	Rationale      string         `json:"rationale"`
	TestsSuggested []string       `json:"tests_suggested"`
	RollbackPlan   string         `json:"rollback_plan"`
	CreatedBy      *string        `json:"created_by,omitempty"`
	CreatedAt      string         `json:"created_at"`
	DecidedBy      *string        `json:"decided_by,omitempty"`
	DecidedAt      *string        `json:"decided_at,omitempty"`
}

type ListProposalsResponse struct {
	Items []ProposalItem `json:"items"`
}
