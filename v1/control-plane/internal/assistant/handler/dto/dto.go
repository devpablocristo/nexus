package dto

type QueryRequest struct {
	Query string `json:"query" binding:"required"`
}

type QueryResponse struct {
	Summary string         `json:"summary"`
	Tables  []TablePayload `json:"tables,omitempty"`
	Actions []ActionHint   `json:"actions,omitempty"`
}

type TablePayload struct {
	Title   string              `json:"title"`
	Columns []string            `json:"columns"`
	Rows    []map[string]string `json:"rows"`
}

type ActionHint struct {
	Label      string                 `json:"label"`
	ActionType string                 `json:"action_type"`
	Payload    map[string]interface{} `json:"payload"`
}
