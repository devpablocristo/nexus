package incidents

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	incidentdto "nexus-core/internal/incidents/handler/dto"
	incidentdomain "nexus-core/internal/incidents/usecases/domain"
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
	rg.POST("/incidents", h.create)
	rg.GET("/incidents", h.list)
	rg.GET("/incidents/:id", h.get)
	rg.POST("/incidents/:id/close", h.close)
}

func (h *Handler) create(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req incidentdto.CreateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	out, err := h.uc.Create(c.Request.Context(), mustOrgID(c), actorFromCtx(c), CreateRequest{
		Severity:         req.Severity,
		Title:            req.Title,
		Summary:          req.Summary,
		RelatedActionIDs: req.RelatedActionIDs,
		EvidenceRefs:     req.EvidenceRefs,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toIncidentItem(out))
}

func (h *Handler) list(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := h.uc.List(c.Request.Context(), mustOrgID(c), c.Query("status"), limit)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := incidentdto.ListIncidentsResponse{Items: make([]incidentdto.IncidentItem, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toIncidentItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) get(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), mustOrgID(c), id)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toIncidentItem(out))
}

func (h *Handler) close(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	out, err := h.uc.Close(c.Request.Context(), mustOrgID(c), id, actorFromCtx(c))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toIncidentItem(out))
}

func toIncidentItem(in incidentdomain.Incident) incidentdto.IncidentItem {
	var closed *string
	if in.ClosedAt != nil {
		s := in.ClosedAt.UTC().Format(time.RFC3339)
		closed = &s
	}
	return incidentdto.IncidentItem{
		ID:               in.ID.String(),
		Severity:         string(in.Severity),
		Status:           string(in.Status),
		Title:            in.Title,
		Summary:          in.Summary,
		RelatedActionIDs: in.RelatedActionIDs,
		EvidenceRefs:     in.EvidenceRefs,
		CreatedBy:        in.CreatedBy,
		OpenedAt:         in.OpenedAt.UTC().Format(time.RFC3339),
		ClosedAt:         closed,
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
