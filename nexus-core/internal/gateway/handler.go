package gateway

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdto "nexus-core/internal/gateway/handler/dto"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/internal/shared/authz"
	httperr "nexus-core/pkg/http/errors"
	"nexus-core/pkg/types"
)

type runUsecase interface {
	Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
	Simulate(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.SimulateResponse, error)
}

type Handler struct {
	uc runUsecase
}

func NewHandler(uc runUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/run", h.run)
	rg.POST("/run/simulate", h.simulate)
}

func (h *Handler) run(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeGatewayRun) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeGatewayRun+" scope required")
		return
	}
	var req gwdto.RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}

	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	rid := req.RequestID
	if rid == "" {
		rid = uuid.NewString()
	}

	role := roleFromCtx(c)
	scopes := scopesFromCtx(c)
	timeoutMS := parseTimeoutMS(c.GetHeader("X-Timeout-Ms"))
	idempotencyKey := parseIdempotencyKey(c.GetHeader("Idempotency-Key"))
	resp, err := h.uc.Run(c.Request.Context(), orgID, gwdomain.RunRequest{
		RequestID:      rid,
		ToolName:       req.ToolName,
		Input:          req.Input,
		Context:        req.Context,
		Actor:          actor,
		Role:           role,
		Scopes:         scopes,
		IdempotencyKey: idempotencyKey,
		TimeoutMS:      timeoutMS,
		RequestSource:  "rest",
		AuthMethod:     authMethodFromCtx(c),
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}

	if resp.Status == gwdomain.RunStatusSuccess {
		writeIdempotencyHeader(c, resp.Idempotency.Outcome)
		c.JSON(resp.HTTPStatus, gwdto.RunSuccessResponse{
			RequestID: resp.RequestID,
			Decision:  string(resp.Decision),
			ToolName:  resp.ToolName,
			Status:    string(resp.Status),
			Result:    resp.Result,
			LatencyMS: resp.LatencyMS,
			Idempotency: &gwdto.IdempotencyDTO{
				Present: resp.Idempotency.Present,
				Outcome: string(resp.Idempotency.Outcome),
			},
		})
		return
	}

	if resp.Status == gwdomain.RunStatusBlocked {
		writeIdempotencyHeader(c, resp.Idempotency.Outcome)
		c.JSON(resp.HTTPStatus, gwdto.RunBlockedResponse{
			RequestID: resp.RequestID,
			Decision:  string(resp.Decision),
			Status:    string(resp.Status),
			Reason:    deref(resp.Reason),
			Error:     types.APIError{Code: deref(resp.ErrorCode), Message: deref(resp.ErrorMsg)},
			LatencyMS: resp.LatencyMS,
			Idempotency: &gwdto.IdempotencyDTO{
				Present: resp.Idempotency.Present,
				Outcome: string(resp.Idempotency.Outcome),
			},
		})
		return
	}

	writeIdempotencyHeader(c, resp.Idempotency.Outcome)
	c.JSON(resp.HTTPStatus, gwdto.RunErrorResponse{
		RequestID: resp.RequestID,
		Decision:  string(resp.Decision),
		Status:    string(resp.Status),
		Error:     types.APIError{Code: deref(resp.ErrorCode), Message: deref(resp.ErrorMsg)},
		LatencyMS: resp.LatencyMS,
		Idempotency: &gwdto.IdempotencyDTO{
			Present: resp.Idempotency.Present,
			Outcome: string(resp.Idempotency.Outcome),
		},
	})
}

func (h *Handler) simulate(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeGatewaySimulate) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeGatewaySimulate+" scope required")
		return
	}
	var req gwdto.RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	role := roleFromCtx(c)
	scopes := scopesFromCtx(c)
	rid := req.RequestID
	if rid == "" {
		rid = uuid.NewString()
	}
	resp, err := h.uc.Simulate(c.Request.Context(), orgID, gwdomain.RunRequest{
		RequestID:  rid,
		ToolName:   req.ToolName,
		Input:      req.Input,
		Context:    req.Context,
		Actor:      actor,
		Role:       role,
		Scopes:     scopes,
		AuthMethod: authMethodFromCtx(c),
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(resp.HTTPStatus, gwdto.SimulateResponse{
		RequestID: resp.RequestID,
		Decision:  string(resp.Decision),
		ToolName:  resp.ToolName,
		Status:    string(resp.Status),
		Reason:    deref(resp.Reason),
		Error:     types.APIError{Code: deref(resp.ErrorCode), Message: deref(resp.ErrorMsg)},
		Explain:   resp.Explain,
		LatencyMS: resp.LatencyMS,
	})
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
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func parseTimeoutMS(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func parseIdempotencyKey(raw string) *string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil
	}
	if len(v) > 255 {
		return nil
	}
	return &v
}

func authMethodFromCtx(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyAuthMethod)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func writeIdempotencyHeader(c *gin.Context, outcome gwdomain.IdempotencyOutcome) {
	if outcome == "" {
		return
	}
	c.Header("X-Idempotency-Outcome", string(outcome))
}
