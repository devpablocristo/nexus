package actions

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	actiondto "nexus-saas/internal/actions/handler/dto"
	actiondomain "nexus-saas/internal/actions/usecases/domain"
	"nexus-saas/internal/shared/authz"
	httperr "nexus/pkg/http/errors"
	"nexus/pkg/types"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/actions/apply", h.apply)
	rg.POST("/actions/rollback", h.rollback)
	rg.GET("/actions", h.list)
}

func (h *Handler) apply(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req actiondto.ApplyActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	out, err := h.uc.Apply(c.Request.Context(), mustOrgID(c), actorFromCtx(c), ApplyRequest{
		ScopeType:    req.ScopeType,
		ScopeID:      req.ScopeID,
		ActionType:   req.ActionType,
		Params:       req.Params,
		TTLSeconds:   req.TTLSeconds,
		EvidenceRefs: req.EvidenceRefs,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toActionItem(out))
}

func (h *Handler) rollback(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req actiondto.RollbackActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	id, err := uuid.Parse(req.ActionID)
	if err != nil {
		httperr.BadRequest(c, "invalid action_id")
		return
	}
	out, err := h.uc.Rollback(c.Request.Context(), mustOrgID(c), id, actorFromCtx(c))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toActionItem(out))
}

func (h *Handler) list(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := h.uc.List(c.Request.Context(), mustOrgID(c), ListQuery{
		Status:     c.Query("status"),
		ActionType: c.Query("action_type"),
		Limit:      limit,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := actiondto.ListActionsResponse{Items: make([]actiondto.ActionItem, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toActionItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func toActionItem(a actiondomain.Action) actiondto.ActionItem {
	var rolledAt *string
	if a.RolledBackAt != nil {
		s := a.RolledBackAt.UTC().Format(time.RFC3339)
		rolledAt = &s
	}
	return actiondto.ActionItem{
		ID:           a.ID.String(),
		ScopeType:    string(a.ScopeType),
		ScopeID:      a.ScopeID,
		ActionType:   string(a.ActionType),
		Params:       a.Params,
		TTLSeconds:   a.TTLSeconds,
		Status:       string(a.Status),
		EvidenceRefs: a.EvidenceRefs,
		CreatedBy:    a.CreatedBy,
		CreatedAt:    a.CreatedAt.UTC().Format(time.RFC3339),
		RolledBackAt: rolledAt,
		RolledBackBy: a.RolledBackBy,
	}
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
