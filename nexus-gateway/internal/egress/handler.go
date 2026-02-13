package egress

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-gateway/internal/egress/handler/dto"
	httperr "nexus-gateway/pkg/http/errors"
	"nexus-gateway/pkg/types"
)

type Handler struct{ svc Service }

func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/tools/:name/egress-rules", h.upsert)
	rg.GET("/tools/:name/egress-rules", h.list)
	rg.DELETE("/tools/:name/egress-rules", h.delete)
}

func (h *Handler) upsert(c *gin.Context) {
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
	hosts, err := h.svc.ListRules(c.Request.Context(), mustOrgID(c), c.Param("name"))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(200, gin.H{"items": hosts})
}

func (h *Handler) delete(c *gin.Context) {
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

// centralized error handling via pkg/http/errors
