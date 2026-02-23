package actionengine

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	actiondto "nexus-core/internal/ops/actionengine/handler/dto"
	"nexus-core/internal/shared/authz"
	httperr "nexus-core/pkg/http/errors"
	ginmw "nexus-core/pkg/http/middlewares/gin"
	"nexus-core/pkg/types"
)

type Handler struct {
	engine Engine
}

func NewHandler(engine Engine) *Handler {
	return &Handler{engine: engine}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/admin/actions/dry-run", h.dryRun)
	rg.POST("/admin/actions/apply", h.apply)
	rg.POST("/admin/actions/rollback", h.rollback)
}

func (h *Handler) dryRun(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	req, ok := bindEngineRequest(c)
	if !ok {
		return
	}
	out, err := h.engine.DryRun(c.Request.Context(), mustOrgID(c), actorFromCtx(c), req)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := toResponse(ginmw.RequestIDFromContext(c), out)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) apply(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	req, ok := bindEngineRequest(c)
	if !ok {
		return
	}
	out, err := h.engine.Apply(c.Request.Context(), mustOrgID(c), actorFromCtx(c), req)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := toResponse(ginmw.RequestIDFromContext(c), out)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) rollback(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	req, ok := bindEngineRequest(c)
	if !ok {
		return
	}
	out, err := h.engine.Rollback(c.Request.Context(), mustOrgID(c), actorFromCtx(c), req)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := toResponse(ginmw.RequestIDFromContext(c), out)
	c.JSON(http.StatusOK, resp)
}

func bindEngineRequest(c *gin.Context) (EngineRequest, bool) {
	var req actiondto.ActionEngineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return EngineRequest{}, false
	}
	out := EngineRequest{
		ActionType:      req.ActionType,
		Scope:           req.Scope,
		TTLSeconds:      req.TTLSeconds,
		Params:          req.Params,
		EvidenceRefs:    req.EvidenceRefs,
		ApprovalGranted: req.ApprovalGranted,
		ApprovalComment: req.ApprovalComment,
	}
	if req.IncidentID != nil {
		id, err := uuid.Parse(strings.TrimSpace(*req.IncidentID))
		if err != nil {
			httperr.BadRequest(c, "invalid incident_id")
			return EngineRequest{}, false
		}
		out.IncidentID = &id
	}
	if req.ProposalID != nil {
		id, err := uuid.Parse(strings.TrimSpace(*req.ProposalID))
		if err != nil {
			httperr.BadRequest(c, "invalid proposal_id")
			return EngineRequest{}, false
		}
		out.ProposalID = &id
	}
	return out, true
}

func toResponse(requestID string, in EngineResult) actiondto.ActionEngineResponse {
	var executionID *string
	if in.Execution != nil {
		v := in.Execution.ID.String()
		executionID = &v
	}
	return actiondto.ActionEngineResponse{
		RequestID:        requestID,
		ProposalID:       in.Proposal.ID.String(),
		ExecutionID:      executionID,
		Status:           string(in.Proposal.Status),
		ActionType:       in.Proposal.ActionType,
		IdempotencyKey:   in.IdempotencyKey,
		ScopeHash:        in.ScopeHash,
		ParamsHash:       in.ParamsHash,
		ApprovalRequired: in.ApprovalRequired,
		Replay:           in.Replay,
		Scope:            in.Proposal.Scope,
		Params:           in.Proposal.Params,
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
