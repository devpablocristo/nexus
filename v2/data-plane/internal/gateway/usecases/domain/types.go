package domain

type RunRequest struct {
	RequestID      string
	ToolName       string
	ToolID         string
	IntentID       string
	IdempotencyKey *string
	TimeoutMS      int
	Input          map[string]any
	Context        map[string]any
}

const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
)

const (
	RunStatusSuccess = "success"
	RunStatusError   = "error"
	RunStatusBlocked = "blocked"
)

type RunResponse struct {
	RequestID   string
	Decision    string
	ToolName    string
	Status      string
	Reason      string
	Result      any
	LatencyMS   int64
	HTTPStatus  int
	IntentID    string
	ApprovalID  string
	Idempotency IdempotencyMeta
}
