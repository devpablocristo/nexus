package world

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-core/internal/shared/authz"
	httperr "nexus-core/pkg/http/errors"
	ginmw "nexus-core/pkg/http/middlewares/gin"
	"nexus-core/pkg/types"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/world/runs", h.listRuns)
	rg.GET("/world/state", h.state)
	rg.GET("/world/events", h.events)
	rg.POST("/world/run/create", h.createRun)
	rg.POST("/world/replay", h.replay)
}

func (h *Handler) listRuns(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	cursor := strings.TrimSpace(c.Query("cursor"))
	out, err := h.svc.ListRuns(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), limit, cursor)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) state(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	runID := strings.TrimSpace(c.Query("run_id"))
	if runID == "" {
		httperr.BadRequest(c, "run_id is required")
		return
	}
	var stepID *int64
	if raw := strings.TrimSpace(c.Query("step_id")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			httperr.BadRequest(c, "step_id must be >= 0")
			return
		}
		stepID = &v
	}
	out, err := h.svc.GetState(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), runID, stepID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) events(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	runID := strings.TrimSpace(c.Query("run_id"))
	if runID == "" {
		httperr.BadRequest(c, "run_id is required")
		return
	}
	fromSeq, _ := strconv.ParseInt(c.DefaultQuery("from_seq", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	out, err := h.svc.GetEvents(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), runID, fromSeq, limit)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) createRun(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.svc.CreateRun(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), payload)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) replay(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.svc.Replay(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), payload)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, ok := c.Get(string(types.CtxKeyOrgID))
	if !ok {
		return uuid.Nil
	}
	if id, ok := v.(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
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
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}
