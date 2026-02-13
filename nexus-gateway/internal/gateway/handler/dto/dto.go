package dto

type RunRequest struct {
	RequestID string         `json:"request_id"`
	ToolName  string         `json:"tool_name"`
	Input     map[string]any `json:"input"`
	Context   map[string]any `json:"context"`
}

type RunSuccessResponse struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
	ToolName  string `json:"tool_name"`
	Status    string `json:"status"`
	Result    any    `json:"result"`
	LatencyMS int64  `json:"latency_ms"`
}

type RunBlockedResponse struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	Error     any    `json:"error"`
	LatencyMS int64  `json:"latency_ms"`
}

type RunErrorResponse struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
	Status    string `json:"status"`
	Error     any    `json:"error"`
	LatencyMS int64  `json:"latency_ms"`
}
