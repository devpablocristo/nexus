package dto

type JSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc" binding:"required"`
	ID      any            `json:"id"`
	Method  string         `json:"method" binding:"required"`
	Params  map[string]any `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}
