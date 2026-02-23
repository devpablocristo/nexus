package dto

type AskRequest struct {
	Question   string  `json:"question" binding:"required"`
	IncidentID *string `json:"incident_id,omitempty"`
}

type AskResponse struct {
	Answer             string   `json:"answer"`
	EvidenceRefs       []string `json:"evidence_refs,omitempty"`
	ProposedActionID   *string  `json:"proposed_action_id,omitempty"`
	ProposedActionType *string  `json:"proposed_action_type,omitempty"`
}
