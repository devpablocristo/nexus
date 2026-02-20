package domain

type RunRequest struct {
	RequestID        string
	ToolName         string
	Input            map[string]any
	Context          map[string]any
	Actor            *string
	Role             *string
	Scopes           []string
	IdempotencyKey   *string
	TimeoutMS        int
	RequestSource    string
	AuthMethod       string
	StageDurations   map[string]int64
	TimeoutAtExecute int
	Idempotency      IdempotencyMeta
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
	RequestID   string
	Decision    Decision
	ToolName    string
	Status      RunStatus
	Reason      *string
	Result      any
	ErrorCode   *string
	ErrorMsg    *string
	LatencyMS   int64
	HTTPStatus  int
	Idempotency IdempotencyMeta
}

type SimulateResponse struct {
	RequestID  string
	Decision   Decision
	ToolName   string
	Status     RunStatus
	Reason     *string
	ErrorCode  *string
	ErrorMsg   *string
	LatencyMS  int64
	HTTPStatus int
	Explain    map[string]any
}

type IdempotencyOutcome string

const (
	IdempotencyNew             IdempotencyOutcome = "NEW"
	IdempotencyReplay          IdempotencyOutcome = "REPLAY"
	IdempotencyInProgress      IdempotencyOutcome = "IN_PROGRESS"
	IdempotencyConflict        IdempotencyOutcome = "CONFLICT"
	IdempotencyMissingRequired IdempotencyOutcome = "MISSING_REQUIRED"
	IdempotencySkippedNotWrite IdempotencyOutcome = "SKIPPED_NOT_WRITE"
)

type IdempotencyMeta struct {
	Present bool
	Outcome IdempotencyOutcome
}
