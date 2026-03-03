package tool

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-core/internal/shared/authz"
	"nexus-core/internal/tool/handler/dto"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	httperr "nexus-core/pkg/http/errors"
	"nexus-core/pkg/types"
)

type toolUsecase interface {
	Create(ctx context.Context, orgID uuid.UUID, req CreateRequest) (tooldomain.Tool, error)
	List(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error)
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
	UpdateByName(ctx context.Context, orgID uuid.UUID, name string, patch ToolPatch) (tooldomain.Tool, error)
}

type Handler struct {
	uc toolUsecase
}

func NewHandler(uc toolUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/tools", h.create)
	rg.GET("/tools", h.list)
	rg.GET("/tools/:name", h.get)
	rg.PUT("/tools/:name", h.update)
}

type createToolRequest struct {
	Name           string         `json:"name"`
	Kind           string         `json:"kind"`
	Description    *string        `json:"description"`
	Method         string         `json:"method"`
	URL            string         `json:"url"`
	InputSchema    map[string]any `json:"input_schema"`
	OutputSchema   map[string]any `json:"output_schema"`
	ActionType     string         `json:"action_type"`
	Classification string         `json:"classification"`
	Sensitivity    string         `json:"sensitivity"`
	RiskLevel      int            `json:"risk_level"`
	Enabled        bool           `json:"enabled"`
}

func (h *Handler) create(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeToolsWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeToolsWrite+" scope required")
		return
	}
	var req createToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	orgID := mustOrgID(c)
	created, err := h.uc.Create(c.Request.Context(), orgID, CreateRequest{
		Name:           req.Name,
		Kind:           req.Kind,
		Description:    req.Description,
		Method:         req.Method,
		URL:            req.URL,
		InputSchema:    req.InputSchema,
		OutputSchema:   nilIfEmpty(req.OutputSchema),
		ActionType:     req.ActionType,
		Classification: req.Classification,
		Sensitivity:    req.Sensitivity,
		RiskLevel:      req.RiskLevel,
		Enabled:        req.Enabled,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toResp(created))
}

func (h *Handler) list(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeToolsRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeToolsRead+" scope required")
		return
	}
	orgID := mustOrgID(c)
	items, err := h.uc.List(c.Request.Context(), orgID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	out := dto.ListToolsResponse{Items: make([]dto.ToolResponse, 0, len(items))}
	for _, t := range items {
		out.Items = append(out.Items, toResp(t))
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) get(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeToolsRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeToolsRead+" scope required")
		return
	}
	orgID := mustOrgID(c)
	name := c.Param("name")
	t, err := h.uc.GetByName(c.Request.Context(), orgID, name)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toResp(t))
}

type updateToolRequest struct {
	Description    *string         `json:"description"`
	Method         *string         `json:"method"`
	URL            *string         `json:"url"`
	InputSchema    *map[string]any `json:"input_schema"`
	OutputSchema   *map[string]any `json:"output_schema"`
	ActionType     *string         `json:"action_type"`
	Classification *string         `json:"classification"`
	Sensitivity    *string         `json:"sensitivity"`
	RiskLevel      *int            `json:"risk_level"`
	Enabled        *bool           `json:"enabled"`
}

func (h *Handler) update(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeToolsWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeToolsWrite+" scope required")
		return
	}
	orgID := mustOrgID(c)
	name := c.Param("name")
	var req updateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	var desc **string
	if req.Description != nil {
		desc = &req.Description
	}
	updated, err := h.uc.UpdateByName(c.Request.Context(), orgID, name, ToolPatch{
		Description:    desc,
		Method:         req.Method,
		URL:            req.URL,
		InputSchema:    req.InputSchema,
		OutputSchema:   req.OutputSchema,
		ActionType:     req.ActionType,
		Classification: req.Classification,
		Sensitivity:    req.Sensitivity,
		RiskLevel:      req.RiskLevel,
		Enabled:        req.Enabled,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toResp(updated))
}

func toResp(t tooldomain.Tool) dto.ToolResponse {
	var in map[string]any
	_ = json.Unmarshal(t.InputSchemaJSON, &in)
	var out map[string]any
	if len(t.OutputSchemaJSON) > 0 {
		_ = json.Unmarshal(t.OutputSchemaJSON, &out)
	}
	resp := dto.ToolResponse{
		ID:             t.ID.String(),
		Name:           t.Name,
		Kind:           string(t.Kind),
		Description:    t.Description,
		Method:         t.Method,
		URL:            t.URL,
		InputSchema:    in,
		ActionType:     string(t.ActionType),
		Classification: t.Classification,
		Sensitivity:    t.Sensitivity,
		RiskLevel:      t.RiskLevel,
		Enabled:        t.Enabled,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
	if out != nil {
		resp.OutputSchema = out
	}
	return resp
}

func nilIfEmpty(m map[string]any) map[string]any {
	if m == nil || len(m) == 0 {
		return nil
	}
	return m
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

// centralized error handling via pkg/http/errors
