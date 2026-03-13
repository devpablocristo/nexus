package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdomain "data-plane/internal/gateway/usecases/domain"
	mcpdto "data-plane/internal/mcp/handler/dto"
	"data-plane/internal/shared/authz"
	tooldomain "data-plane/internal/tool/usecases/domain"
	httperr "nexus/pkg/http/errors"
	ginmw "nexus/pkg/http/middlewares/gin"
	"nexus/pkg/types"
)

type mcpUsecase interface {
	ListTools(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error)
	GetTool(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
	CallTool(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type Handler struct{ uc mcpUsecase }

func NewHandler(uc mcpUsecase) *Handler { return &Handler{uc: uc} }

func AsMCPUsecase(uc *Usecases) mcpUsecase { return uc }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/mcp", h.rpc)
}

func (h *Handler) rpc(c *gin.Context) {
	var req mcpdto.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, mcpdto.JSONRPCResponse{JSONRPC: "2.0", ID: nil, Error: &mcpdto.JSONRPCError{Code: -32600, Message: "invalid request", Data: map[string]any{"request_id": ginmw.RequestIDFromContext(c), "error_code": types.ErrCodeValidation}}})
		return
	}
	orgID := mustOrgID(c)
	switch req.Method {
	case "tools/list":
		if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeMCPRead) {
			h.rpcErr(c, req.ID, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeMCPRead+" scope required"))
			return
		}
		items, err := h.uc.ListTools(c.Request.Context(), orgID)
		if err != nil {
			h.rpcErr(c, req.ID, err)
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, t := range items {
			out = append(out, map[string]any{"name": t.Name, "kind": t.Kind, "method": t.Method, "url": t.URL, "input_schema": jsonRawToAny(t.InputSchemaJSON), "output_schema": jsonRawToAny(t.OutputSchemaJSON), "action_type": t.ActionType, "classification": t.Classification, "sensitivity": t.Sensitivity, "risk_level": t.RiskLevel, "enabled": t.Enabled})
		}
		c.JSON(http.StatusOK, mcpdto.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"items": out}})
	case "tools/get":
		if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeMCPRead) {
			h.rpcErr(c, req.ID, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeMCPRead+" scope required"))
			return
		}
		name, _ := req.Params["tool_name"].(string)
		tool, err := h.uc.GetTool(c.Request.Context(), orgID, name)
		if err != nil {
			h.rpcErr(c, req.ID, err)
			return
		}
		c.JSON(http.StatusOK, mcpdto.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"name": tool.Name, "kind": tool.Kind, "method": tool.Method, "url": tool.URL, "input_schema": jsonRawToAny(tool.InputSchemaJSON), "output_schema": jsonRawToAny(tool.OutputSchemaJSON), "action_type": tool.ActionType, "classification": tool.Classification, "sensitivity": tool.Sensitivity, "risk_level": tool.RiskLevel, "enabled": tool.Enabled}})
	case "tools/call":
		if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeMCPCall) {
			h.rpcErr(c, req.ID, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeMCPCall+" scope required"))
			return
		}
		toolName, _ := req.Params["tool_name"].(string)
		input, _ := req.Params["input"].(map[string]any)
		ctxMap, _ := req.Params["context"].(map[string]any)
		rid, _ := req.Params["request_id"].(string)
		idempotencyKey, _ := req.Params["idempotency_key"].(string)
		timeoutMS := intFromAny(req.Params["timeout_ms"])
		if timeoutMS == 0 {
			timeoutMS = parseTimeoutHeader(c.GetHeader("X-Timeout-Ms"))
		}
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		}
		actor := actorFromCtx(c)
		role := roleFromCtx(c)
		scopes := scopesFromCtx(c)
		resp, err := h.uc.CallTool(c.Request.Context(), orgID, gwdomain.RunRequest{
			RequestID:      rid,
			ToolName:       toolName,
			Input:          input,
			Context:        ctxMap,
			Actor:          actor,
			Role:           role,
			Scopes:         scopes,
			IdempotencyKey: strPtrIfNonEmpty(idempotencyKey),
			TimeoutMS:      timeoutMS,
			RequestSource:  "mcp",
			AuthMethod:     authMethodFromCtx(c),
		})
		if err != nil {
			h.rpcErr(c, req.ID, err)
			return
		}
		c.JSON(http.StatusOK, mcpdto.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"request_id": resp.RequestID, "decision": resp.Decision, "status": resp.Status, "tool_name": resp.ToolName, "reason": resp.Reason, "result": resp.Result, "error": map[string]any{"code": deref(resp.ErrorCode), "message": deref(resp.ErrorMsg)}, "latency_ms": resp.LatencyMS, "idempotency": map[string]any{"present": resp.Idempotency.Present, "outcome": resp.Idempotency.Outcome}}})
	default:
		c.JSON(http.StatusOK, mcpdto.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpdto.JSONRPCError{Code: -32601, Message: "method not found", Data: map[string]any{"request_id": ginmw.RequestIDFromContext(c), "error_code": types.ErrCodeNotFound}}})
	}
}

func (h *Handler) rpcErr(c *gin.Context, id any, err error) {
	requestID := ginmw.RequestIDFromContext(c)
	status, apiErr := httperr.Normalize(err)
	code := -32000
	if status >= 500 {
		code = -32603
	}
	c.JSON(http.StatusOK, mcpdto.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpdto.JSONRPCError{
			Code:    code,
			Message: apiErr.Message,
			Data: map[string]any{
				"request_id":  requestID,
				"error_code":  apiErr.Code,
				"http_status": status,
			},
		},
	})
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(string(types.CtxKeyOrgID))
	id, _ := v.(uuid.UUID)
	return id
}

func actorFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyActor)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func roleFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyRole)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func scopesFromCtx(c *gin.Context) []string {
	if v, ok := c.Get(string(types.CtxKeyScopes)); ok {
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func jsonRawToAny(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func strPtrIfNonEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func authMethodFromCtx(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyAuthMethod)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func parseTimeoutHeader(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
