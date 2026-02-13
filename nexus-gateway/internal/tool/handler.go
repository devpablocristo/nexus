package tool

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-gateway/internal/tool/handler/dto"
	tooldomain "nexus-gateway/internal/tool/usecases/domain"
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
	var req createToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	orgID := mustOrgID(c)
	created, err := h.svc.Create(c.Request.Context(), orgID, CreateRequest{
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
	orgID := mustOrgID(c)
	items, err := h.svc.List(c.Request.Context(), orgID)
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
	orgID := mustOrgID(c)
	name := c.Param("name")
	t, err := h.svc.GetByName(c.Request.Context(), orgID, name)
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
	updated, err := h.svc.UpdateByName(c.Request.Context(), orgID, name, ToolPatch{
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

// centralized error handling via pkg/http/errors
