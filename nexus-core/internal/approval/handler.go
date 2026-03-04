package approval

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	approvaldto "nexus-core/internal/approval/handler/dto"
	domain "nexus-core/internal/approval/usecases/domain"
	"nexus-core/internal/shared/authz"
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
	rg.GET("/approvals", h.listPending)
	rg.GET("/approvals/:id", h.getByID)
	rg.POST("/approvals/:id/approve", h.approve)
	rg.POST("/approvals/:id/reject", h.reject)
}

func (h *Handler) listPending(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	items, err := h.uc.ListPending(c.Request.Context(), mustOrgID(c), 100)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := approvaldto.ListApprovalsResponse{Items: make([]approvaldto.ApprovalItem, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toDTO(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) getByID(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	item, err := h.uc.GetByID(c.Request.Context(), mustOrgID(c), id)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toDTO(item))
}

func (h *Handler) approve(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	var req approvaldto.DecideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	decidedBy := req.DecidedBy
	if decidedBy == "" {
		if v := actorFromCtx(c); v != nil {
			decidedBy = *v
		}
	}
	if err := h.uc.Approve(c.Request.Context(), mustOrgID(c), id, decidedBy); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}

func (h *Handler) reject(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	var req approvaldto.DecideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	decidedBy := req.DecidedBy
	if decidedBy == "" {
		if v := actorFromCtx(c); v != nil {
			decidedBy = *v
		}
	}
	if err := h.uc.Reject(c.Request.Context(), mustOrgID(c), id, decidedBy); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

func toDTO(a domain.PendingApproval) approvaldto.ApprovalItem {
	return approvaldto.ApprovalItem{
		ID:              a.ID.String(),
		RequestID:       a.RequestID,
		ToolName:        a.ToolName,
		Actor:           a.Actor,
		Role:            a.Role,
		InputRedacted:   a.InputRedacted,
		ContextRedacted: a.ContextRedacted,
		Reason:          a.Reason,
		Status:          string(a.Status),
		DecidedBy:       a.DecidedBy,
		DecidedAt:       a.DecidedAt,
		ExpiresAt:       a.ExpiresAt,
		CreatedAt:       a.CreatedAt,
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
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}
