package a2a

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	a2adto "nexus-gateway/internal/a2a/handler/dto"
	gwdomain "nexus-gateway/internal/gateway/usecases/domain"
	"nexus-gateway/internal/shared/authz"
	httperr "nexus-gateway/pkg/http/errors"
	"nexus-gateway/pkg/types"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/a2a/call", h.call)
}

func (h *Handler) call(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeA2ACall) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeA2ACall+" scope required")
		return
	}
	var req a2adto.CallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}

	timeoutMS := req.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = parseTimeoutMS(c.GetHeader("X-Timeout-Ms"))
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}

	resp, err := h.svc.CallTool(c.Request.Context(), mustOrgID(c), gwdomain.RunRequest{
		RequestID:      req.RequestID,
		ToolName:       req.ToolName,
		Input:          req.Input,
		Context:        req.Context,
		Actor:          actorFromCtx(c),
		Role:           roleFromCtx(c),
		Scopes:         scopesFromCtx(c),
		IdempotencyKey: strPtrIfNonEmpty(idempotencyKey),
		TimeoutMS:      timeoutMS,
		RequestSource:  "a2a",
		AuthMethod:     authMethodFromCtx(c),
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}

	out := a2adto.CallResponse{
		RequestID: resp.RequestID,
		Decision:  string(resp.Decision),
		ToolName:  resp.ToolName,
		Status:    string(resp.Status),
		Result:    resp.Result,
		LatencyMS: resp.LatencyMS,
		Error: a2adto.ErrorObj{
			Code:    deref(resp.ErrorCode),
			Message: deref(resp.ErrorMsg),
		},
		Idempotency: &a2adto.Idempotency{
			Present: resp.Idempotency.Present,
			Outcome: string(resp.Idempotency.Outcome),
		},
	}
	if resp.Reason != nil {
		out.Reason = *resp.Reason
	}
	c.JSON(resp.HTTPStatus, out)
}

func parseTimeoutMS(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func strPtrIfNonEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
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

func authMethodFromCtx(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyAuthMethod)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
