package dto

type RunRequest struct {
	RequestID string         `json:"request_id"`
	ToolName  string         `json:"tool_name"`
	Input     map[string]any `json:"input"`
	Context   map[string]any `json:"context"`
}

type RunSuccessResponse struct {
	RequestID   string          `json:"request_id"`
	Decision    string          `json:"decision"`
	ToolName    string          `json:"tool_name"`
	Status      string          `json:"status"`
	Result      any             `json:"result"`
	LatencyMS   int64           `json:"latency_ms"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
}

type RunBlockedResponse struct {
	RequestID   string          `json:"request_id"`
	Decision    string          `json:"decision"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason"`
	Error       any             `json:"error"`
	LatencyMS   int64           `json:"latency_ms"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
}

type RunErrorResponse struct {
	RequestID   string          `json:"request_id"`
	Decision    string          `json:"decision"`
	Status      string          `json:"status"`
	Error       any             `json:"error"`
	LatencyMS   int64           `json:"latency_ms"`
	Idempotency *IdempotencyDTO `json:"idempotency,omitempty"`
}

type SimulateResponse struct {
	RequestID string         `json:"request_id"`
	Decision  string         `json:"decision"`
	ToolName  string         `json:"tool_name"`
	Status    string         `json:"status"`
	Reason    string         `json:"reason,omitempty"`
	Error     any            `json:"error,omitempty"`
	Explain   map[string]any `json:"explain"`
	LatencyMS int64          `json:"latency_ms"`
}

type IdempotencyDTO struct {
	Present bool   `json:"present"`
	Outcome string `json:"outcome"`
}
