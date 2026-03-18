package dto

type SubmitRequest struct {
	RequesterType  string         `json:"requester_type"`
	RequesterID    string         `json:"requester_id"`
	RequesterName  string         `json:"requester_name,omitempty"`
	ActionType     string         `json:"action_type"`
	TargetSystem   string         `json:"target_system,omitempty"`
	TargetResource string         `json:"target_resource,omitempty"`
	Params         map[string]any `json:"params,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	Context        string         `json:"context,omitempty"`
}

type SubmitResponse struct {
	RequestID      string           `json:"request_id"`
	Decision       string           `json:"decision"`
	RiskLevel      string           `json:"risk_level"`
	DecisionReason string           `json:"decision_reason"`
	Status         string           `json:"status"`
	Approval       *ApprovalPayload `json:"approval,omitempty"`
	AISummary      string           `json:"ai_summary,omitempty"`
	AIDegraded     bool             `json:"ai_degraded,omitempty"`
}

type ApprovalPayload struct {
	ID        string `json:"id"`
	ExpiresAt string `json:"expires_at"`
}

type ReportResultRequest struct {
	Success      bool           `json:"success"`
	Result       map[string]any `json:"result,omitempty"`
	DurationMs   int64          `json:"duration_ms,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
}

type RequestResponse struct {
	ID             string         `json:"id"`
	RequesterType  string         `json:"requester_type"`
	RequesterID    string         `json:"requester_id"`
	RequesterName  string         `json:"requester_name,omitempty"`
	ActionType     string         `json:"action_type"`
	TargetSystem   string         `json:"target_system,omitempty"`
	TargetResource string         `json:"target_resource,omitempty"`
	Params         map[string]any `json:"params,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	RiskLevel      string         `json:"risk_level"`
	Decision       string         `json:"decision"`
	DecisionReason string         `json:"decision_reason,omitempty"`
	Status         string         `json:"status"`
	AISummary      string         `json:"ai_summary,omitempty"`
	AIDegraded     bool           `json:"ai_degraded,omitempty"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}
