package assistant

import (
	"net/http"

	"github.com/gin-gonic/gin"

	assistantdto "nexus-saas/internal/assistant/handler/dto"
	hlp "nexus-saas/internal/assistant/handler"
	"nexus-saas/internal/shared/authz"
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
	rg.POST("/assistant/query", h.query)
	rg.POST("/assistant/tick", h.tick)
}

func (h *Handler) query(c *gin.Context) {
	if !authz.CanAccess(hlp.ScopesFromCtx(c), hlp.RoleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	var req assistantdto.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	out, err := h.uc.Query(c.Request.Context(), hlp.MustOrgID(c), hlp.ActorFromCtx(c), req.Query)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := assistantdto.QueryResponse{Summary: out.Summary}
	for _, t := range out.Tables {
		resp.Tables = append(resp.Tables, assistantdto.TablePayload{
			Title:   t.Title,
			Columns: t.Columns,
			Rows:    t.Rows,
		})
	}
	for _, a := range out.Actions {
		resp.Actions = append(resp.Actions, assistantdto.ActionHint{
			Label:      a.Label,
			ActionType: a.ActionType,
			Payload:    a.Payload,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) tick(c *gin.Context) {
	if !authz.CanAccess(hlp.ScopesFromCtx(c), hlp.RoleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	if err := h.uc.Tick(c.Request.Context()); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
