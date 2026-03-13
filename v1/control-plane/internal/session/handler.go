package session

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sessiondto "control-plane/internal/session/handler/dto"
	domain "control-plane/internal/session/usecases/domain"
	"control-plane/internal/shared/authz"
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
	rg.GET("/sessions/:session_id", h.getSession)
}

func (h *Handler) getSession(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	sessionID := c.Param("session_id")
	if sessionID == "" {
		httperr.BadRequest(c, "session_id required")
		return
	}
	sess, err := h.uc.GetBySessionID(c.Request.Context(), mustOrgID(c), sessionID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toDTO(sess))
}

func toDTO(s domain.AgentSession) sessiondto.SessionItem {
	return sessiondto.SessionItem{
		ID:           s.ID.String(),
		SessionID:    s.SessionID,
		Actor:        s.Actor,
		TotalCalls:   s.TotalCalls,
		TotalWrites:  s.TotalWrites,
		TotalDenials: s.TotalDenials,
		Metadata:     s.Metadata,
		CreatedAt:    s.CreatedAt,
		LastCallAt:   s.LastCallAt,
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
