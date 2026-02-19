package dto

type CallRequest struct {
	RequestID      string         `json:"request_id"`
	ToolName       string         `json:"tool_name" binding:"required"`
	Input          map[string]any `json:"input"`
	Context        map[string]any `json:"context"`
	TimeoutMS      int            `json:"timeout_ms"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type Idempotency struct {
	Present bool   `json:"present"`
	Outcome string `json:"outcome"`
}

type CallResponse struct {
	RequestID   string       `json:"request_id"`
	Decision    string       `json:"decision"`
	ToolName    string       `json:"tool_name"`
	Status      string       `json:"status"`
	Reason      string       `json:"reason,omitempty"`
	Result      any          `json:"result,omitempty"`
	Error       ErrorObj     `json:"error"`
	LatencyMS   int64        `json:"latency_ms"`
	Idempotency *Idempotency `json:"idempotency,omitempty"`
}

type ErrorObj struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
