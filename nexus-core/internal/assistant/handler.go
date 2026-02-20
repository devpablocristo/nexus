package assistant

import (
	"net/http"

	"github.com/gin-gonic/gin"

	assistantdto "nexus-core/internal/assistant/handler/dto"
	"nexus-core/internal/shared/authz"
	httperr "nexus-core/pkg/http/errors"
	"nexus-core/pkg/types"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/assistant/query", h.query)
}

func (h *Handler) query(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	var req assistantdto.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	out, err := h.svc.Query(c.Request.Context(), mustOrgID(c), actorFromCtx(c), req.Query)
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
