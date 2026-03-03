package policyproposal

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	proposaldto "nexus-core/internal/policyproposal/handler/dto"
	proposaldomain "nexus-core/internal/policyproposal/usecases/domain"
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
	rg.POST("/policy-proposals", h.create)
	rg.GET("/policy-proposals", h.list)
	rg.POST("/policy-proposals/:id/approve", h.approve)
	rg.POST("/policy-proposals/:id/reject", h.reject)
	rg.POST("/policy-proposals/:id/shadow", h.shadow)
}

func (h *Handler) create(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req proposaldto.CreateProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	out, err := h.uc.Create(c.Request.Context(), mustOrgID(c), actorFromCtx(c), CreateRequest{
		Status:         req.Status,
		Diff:           req.Diff,
		Rationale:      req.Rationale,
		TestsSuggested: req.TestsSuggested,
		RollbackPlan:   req.RollbackPlan,
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProposalItem(out))
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
	resp := proposaldto.ListProposalsResponse{Items: make([]proposaldto.ProposalItem, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toProposalItem(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) approve(c *gin.Context) { h.decide(c, "approve") }
func (h *Handler) reject(c *gin.Context)  { h.decide(c, "reject") }
func (h *Handler) shadow(c *gin.Context)  { h.decide(c, "shadow") }

func (h *Handler) decide(c *gin.Context, kind string) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	var out proposaldomain.Proposal
	switch kind {
	case "approve":
		out, err = h.uc.Approve(c.Request.Context(), mustOrgID(c), id, actorFromCtx(c))
	case "reject":
		out, err = h.uc.Reject(c.Request.Context(), mustOrgID(c), id, actorFromCtx(c))
	default:
		out, err = h.uc.Shadow(c.Request.Context(), mustOrgID(c), id, actorFromCtx(c))
	}
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toProposalItem(out))
}

func toProposalItem(p proposaldomain.Proposal) proposaldto.ProposalItem {
	var decidedAt *string
	if p.DecidedAt != nil {
		s := p.DecidedAt.UTC().Format(time.RFC3339)
		decidedAt = &s
	}
	return proposaldto.ProposalItem{
		ID:             p.ID.String(),
		Status:         string(p.Status),
		Diff:           p.Diff,
		Rationale:      p.Rationale,
		TestsSuggested: p.TestsSuggested,
		RollbackPlan:   p.RollbackPlan,
		CreatedBy:      p.CreatedBy,
		CreatedAt:      p.CreatedAt.UTC().Format(time.RFC3339),
		DecidedBy:      p.DecidedBy,
		DecidedAt:      decidedAt,
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
