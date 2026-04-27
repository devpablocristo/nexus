package dto

// ProposalDecisionRequest representa el body de accept/dismiss.
type ProposalDecisionRequest struct {
	DecidedBy string `json:"decided_by"`
}

// ProposalResponse representa una propuesta en la respuesta HTTP.
type ProposalResponse struct {
	ID                  string  `json:"id"`
	ProposedName        string  `json:"proposed_name"`
	ProposedDescription string  `json:"proposed_description,omitempty"`
	ProposedExpression  string  `json:"proposed_expression"`
	ProposedEffect      string  `json:"proposed_effect"`
	ProposedActionType  *string `json:"proposed_action_type,omitempty"`
	ProposedPriority    int     `json:"proposed_priority"`
	PatternSummary      string  `json:"pattern_summary"`
	Confidence          float64 `json:"confidence"`
	SampleSize          int     `json:"sample_size"`
	TimeWindow          string  `json:"time_window"`
	Status              string  `json:"status"`
	DecidedBy           *string `json:"decided_by,omitempty"`
	DecidedAt           *string `json:"decided_at,omitempty"`
	PolicyID            *string `json:"policy_id,omitempty"`
	CreatedAt           string  `json:"created_at"`
}
