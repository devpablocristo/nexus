package dto

type CreateIncidentRequest struct {
	Severity         string   `json:"severity" binding:"required"`
	Title            string   `json:"title" binding:"required"`
	Summary          string   `json:"summary" binding:"required"`
	RelatedActionIDs []string `json:"related_action_ids"`
	EvidenceRefs     []string `json:"evidence_refs"`
}

type IncidentItem struct {
	ID               string   `json:"id"`
	Severity         string   `json:"severity"`
	Status           string   `json:"status"`
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	RelatedActionIDs []string `json:"related_action_ids"`
	EvidenceRefs     []string `json:"evidence_refs"`
	CreatedBy        *string  `json:"created_by,omitempty"`
	OpenedAt         string   `json:"opened_at"`
	ClosedAt         *string  `json:"closed_at,omitempty"`
}

type ListIncidentsResponse struct {
	Items []IncidentItem `json:"items"`
}
