package events

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	eventdto "nexus-control-operators/internal/events/handler/dto"
	"nexus-control-operators/internal/shared/authz"
	httperr "nexus-control-operators/pkg/http/errors"
	"nexus-control-operators/pkg/types"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/events", h.list)
}

func (h *Handler) list(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAuditRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAuditRead+" scope required")
		return
	}
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, next, err := h.uc.ListByCursor(c.Request.Context(), mustOrgID(c), cursor, limit)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := eventdto.ListEventsResponse{Items: make([]eventdto.EventItem, 0, len(items)), NextCursor: next}
	for _, it := range items {
		resp.Items = append(resp.Items, eventdto.EventItem{
			ID:        it.ID,
			EventType: it.EventType,
			Payload:   it.Payload,
			CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, resp)
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
