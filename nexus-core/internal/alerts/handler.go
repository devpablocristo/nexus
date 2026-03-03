package alerts

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	alertdto "nexus-core/internal/alerts/handler/dto"
	domain "nexus-core/internal/alerts/usecases/domain"
	"nexus-core/internal/shared/authz"
	httperr "nexus-core/pkg/http/errors"
	"nexus-core/pkg/types"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/alert-rules", h.list)
	rg.POST("/alert-rules", h.create)
	rg.DELETE("/alert-rules/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	rules, err := h.uc.ListByOrg(c.Request.Context(), mustOrgID(c))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := alertdto.ListAlertRulesResponse{Items: make([]alertdto.AlertRuleItem, 0, len(rules))}
	for _, r := range rules {
		resp.Items = append(resp.Items, toDTO(r))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) create(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req alertdto.CreateAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	rule := domain.AlertRule{
		OrgID:           mustOrgID(c),
		Name:            req.Name,
		Metric:          domain.Metric(req.Metric),
		Threshold:       req.Threshold,
		WindowSeconds:   req.WindowSeconds,
		ToolName:        req.ToolName,
		WebhookURL:      req.WebhookURL,
		CooldownSeconds: req.CooldownSeconds,
		Enabled:         req.Enabled,
	}
	created, err := h.uc.Create(c.Request.Context(), rule)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toDTO(created))
}

func (h *Handler) delete(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	if err := h.uc.Delete(c.Request.Context(), mustOrgID(c), id); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func toDTO(r domain.AlertRule) alertdto.AlertRuleItem {
	return alertdto.AlertRuleItem{
		ID:              r.ID.String(),
		Name:            r.Name,
		Metric:          string(r.Metric),
		Threshold:       r.Threshold,
		WindowSeconds:   r.WindowSeconds,
		ToolName:        r.ToolName,
		WebhookURL:      r.WebhookURL,
		CooldownSeconds: r.CooldownSeconds,
		Enabled:         r.Enabled,
		LastFiredAt:     r.LastFiredAt,
		CreatedAt:       r.CreatedAt,
	}
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
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}
