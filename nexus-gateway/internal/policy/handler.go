package policy

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-gateway/internal/policy/handler/dto"
	policydomain "nexus-gateway/internal/policy/usecases/domain"
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
	rg.POST("/tools/:name/policies", h.createForTool)
	rg.GET("/tools/:name/policies", h.listForTool)
	rg.PUT("/policies/:id", h.updateByID)
}

func (h *Handler) createForTool(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopePolicyWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopePolicyWrite+" scope required")
		return
	}
	orgID := mustOrgID(c)
	toolName := c.Param("name")
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	created, err := h.svc.CreateForTool(c.Request.Context(), orgID, toolName, req)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(201, toResp(created))
}

func (h *Handler) listForTool(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopePolicyRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopePolicyRead+" scope required")
		return
	}
	orgID := mustOrgID(c)
	toolName := c.Param("name")
	items, err := h.svc.ListForTool(c.Request.Context(), orgID, toolName)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	out := dto.ListPoliciesResponse{Items: make([]dto.PolicyResponse, 0, len(items))}
	for _, p := range items {
		out.Items = append(out.Items, toResp(p))
	}
	c.JSON(200, out)
}

func (h *Handler) updateByID(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopePolicyWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopePolicyWrite+" scope required")
		return
	}
	orgID := mustOrgID(c)
	idStr := c.Param("id")
	pid, err := uuid.Parse(idStr)
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	var req struct {
		Effect         *string         `json:"effect"`
		Priority       *int            `json:"priority"`
		Conditions     *map[string]any `json:"conditions"`
		Limits         *map[string]any `json:"limits"`
		ReasonTemplate *string         `json:"reason_template"`
		Enabled        *bool           `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	updated, err := h.svc.UpdateByID(c.Request.Context(), orgID, pid, PolicyPatch{
		Effect:         req.Effect,
		Priority:       req.Priority,
		Conditions:     req.Conditions,
		Limits:         req.Limits,
		ReasonTemplate: req.ReasonTemplate,
		Enabled:        req.Enabled,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(200, toResp(updated))
}

func toResp(p policydomain.Policy) dto.PolicyResponse {
	var cond map[string]any
	_ = json.Unmarshal(p.ConditionsJSON, &cond)
	var lim map[string]any
	_ = json.Unmarshal(p.LimitsJSON, &lim)
	return dto.PolicyResponse{
		ID:             p.ID.String(),
		ToolID:         p.ToolID.String(),
		Effect:         string(p.Effect),
		Priority:       p.Priority,
		Conditions:     cond,
		Limits:         lim,
		ReasonTemplate: p.ReasonTemplate,
		Enabled:        p.Enabled,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
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
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}

// centralized error handling via pkg/http/errors
