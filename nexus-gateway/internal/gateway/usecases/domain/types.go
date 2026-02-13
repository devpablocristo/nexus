package domain

type RunRequest struct {
	RequestID string
	ToolName  string
	Input     map[string]any
	Context   map[string]any
}

type RunStatus string

const (
	RunStatusSuccess RunStatus = "success"
	RunStatusError   RunStatus = "error"
	RunStatusBlocked RunStatus = "blocked"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

type RunResponse struct {
	RequestID  string
	Decision   Decision
	ToolName   string
	Status     RunStatus
	Reason     *string
	Result     any
	ErrorCode  *string
	ErrorMsg   *string
	LatencyMS  int64
	HTTPStatus int
}
