package executive_qa

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	exqdto "nexus-core/internal/agents/executive_qa/handler/dto"
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
	rg.POST("/admin/executive-qa/ask", h.ask)
}

func (h *Handler) ask(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req exqdto.AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	var incidentID *uuid.UUID
	if req.IncidentID != nil && strings.TrimSpace(*req.IncidentID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.IncidentID))
		if err != nil {
			httperr.BadRequest(c, "invalid incident_id")
			return
		}
		incidentID = &id
	}
	out, err := h.uc.Ask(c.Request.Context(), mustOrgID(c), actorFromCtx(c), AskRequest{
		Question:   req.Question,
		IncidentID: incidentID,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, exqdto.AskResponse{
		Answer:             out.Answer,
		EvidenceRefs:       out.EvidenceRefs,
		ProposedActionID:   out.ProposedActionID,
		ProposedActionType: out.ProposedActionType,
	})
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
