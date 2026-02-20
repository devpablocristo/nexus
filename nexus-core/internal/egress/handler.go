package egress

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-core/internal/egress/handler/dto"
	"nexus-core/internal/shared/authz"
	httperr "nexus-core/pkg/http/errors"
	"nexus-core/pkg/types"
)

type Handler struct{ svc Service }

func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/tools/:name/egress-rules", h.upsert)
	rg.GET("/tools/:name/egress-rules", h.list)
	rg.DELETE("/tools/:name/egress-rules", h.delete)
}

func (h *Handler) upsert(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeEgressWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeEgressWrite+" scope required")
		return
	}
	var req dto.UpsertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.svc.UpsertRule(c.Request.Context(), mustOrgID(c), c.Param("name"), req.Host, enabled); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.Status(204)
}

func (h *Handler) list(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeEgressRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeEgressRead+" scope required")
		return
	}
	hosts, err := h.svc.ListRules(c.Request.Context(), mustOrgID(c), c.Param("name"))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(200, gin.H{"items": hosts})
}

func (h *Handler) delete(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeEgressWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeEgressWrite+" scope required")
		return
	}
	if err := h.svc.DeleteRule(c.Request.Context(), mustOrgID(c), c.Param("name"), c.Query("host")); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.Status(204)
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(string(types.CtxKeyOrgID))
	id, _ := v.(uuid.UUID)
	return id
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

// centralized error handling via pkg/http/errors
