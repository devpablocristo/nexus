package gateway

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	gwdto "data-plane/internal/gateway/handler/dto"
	gwdomain "data-plane/internal/gateway/usecases/domain"
	"data-plane/internal/shared/authz"
	httperr "nexus/pkg/http/errors"
	"nexus/pkg/types"
)

type runUsecase interface {
	Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
	Simulate(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.SimulateResponse, error)
	ExecuteIntent(ctx context.Context, orgID, intentID uuid.UUID, timeoutMS int) (gwdomain.RunResponse, error)
	ExecuteIntentWithLease(ctx context.Context, orgID, intentID, leaseID uuid.UUID, timeoutMS int) (gwdomain.RunResponse, error)
	IssueExecutionLease(ctx context.Context, orgID, intentID uuid.UUID) (gwdomain.ExecutionLease, error)
	GetIntent(ctx context.Context, orgID, intentID uuid.UUID) (gwdomain.ExecutionIntent, error)
	ListIntents(ctx context.Context, orgID uuid.UUID, limit int) ([]gwdomain.ExecutionIntent, error)
	GetIntentPreflight(ctx context.Context, orgID, intentID uuid.UUID) (gwdomain.PreflightReview, error)
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
	rg.GET("/run/intents", h.listIntents)
	rg.GET("/run/intents/:id", h.getIntent)
	rg.GET("/run/intents/:id/preflight", h.getIntentPreflight)
	rg.POST("/run/intents/:id/lease", h.issueExecutionLease)
	rg.POST("/run/intents/:id/execute", h.executeIntent)
}

func (h *Handler) run(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeGatewayRun) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeGatewayRun+" scope required")
		return
	}
	var req gwdto.RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, runRequestBindErrorMessage(err))
		return
	}
	if strings.TrimSpace(req.ToolName) == "" && strings.TrimSpace(req.ToolID) == "" {
		httperr.BadRequest(c, "tool_name or tool_id required")
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
		ToolID:         req.ToolID,
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
	writeRunResponse(c, resp)
}

func (h *Handler) simulate(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeGatewaySimulate) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeGatewaySimulate+" scope required")
		return
	}
	var req gwdto.RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, runRequestBindErrorMessage(err))
		return
	}
	if strings.TrimSpace(req.ToolName) == "" && strings.TrimSpace(req.ToolID) == "" {
		httperr.BadRequest(c, "tool_name or tool_id required")
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
		ToolID:     req.ToolID,
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

func (h *Handler) executeIntent(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeGatewayRun) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeGatewayRun+" scope required")
		return
	}
	intentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	var req gwdto.ExecuteIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "lease_id required")
		return
	}
	leaseID, err := uuid.Parse(req.LeaseID)
	if err != nil {
		httperr.BadRequest(c, "invalid lease_id")
		return
	}
	resp, err := h.uc.ExecuteIntentWithLease(c.Request.Context(), mustOrgID(c), intentID, leaseID, parseTimeoutMS(c.GetHeader("X-Timeout-Ms")))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	writeRunResponse(c, resp)
}

func (h *Handler) issueExecutionLease(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeGatewayRun) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeGatewayRun+" scope required")
		return
	}
	intentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	lease, err := h.uc.IssueExecutionLease(c.Request.Context(), mustOrgID(c), intentID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toExecutionLeaseDTO(lease))
}

func (h *Handler) listIntents(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := h.uc.ListIntents(c.Request.Context(), mustOrgID(c), limit)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := gwdto.ListIntentsResponse{Items: make([]gwdto.IntentItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toIntentDTO(item))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) getIntent(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	intentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	item, err := h.uc.GetIntent(c.Request.Context(), mustOrgID(c), intentID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toIntentDTO(item))
}

func (h *Handler) getIntentPreflight(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	intentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	item, err := h.uc.GetIntentPreflight(c.Request.Context(), mustOrgID(c), intentID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toPreflightReviewDTO(item))
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

func toExecutionLeaseDTO(lease gwdomain.ExecutionLease) gwdto.ExecutionLeaseItem {
	return gwdto.ExecutionLeaseItem{
		ID:              lease.ID.String(),
		IntentID:        lease.IntentID.String(),
		ToolName:        lease.ToolName,
		RiskClass:       string(lease.RiskClass),
		Status:          string(lease.Status),
		CredentialMode:  lease.CredentialMode,
		CredentialHints: cloneMap(lease.CredentialHints),
		ExpiresAt:       lease.ExpiresAt,
		UsedAt:          lease.UsedAt,
		CreatedAt:       lease.CreatedAt,
	}
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

func runRequestBindErrorMessage(err error) string {
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		missingInput := false
		for _, ferr := range validationErrs {
			switch ferr.Field() {
			case "Input":
				if ferr.Tag() == "required" {
					missingInput = true
				}
			}
		}
		if missingInput {
			return "input required"
		}
	}

	// Gin may return validator output as plain text; keep compatibility with existing e2e assertions.
	raw := err.Error()
	if strings.Contains(raw, "RunRequest.Input") {
		return "input required"
	}
	return "invalid json"
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

func writeRunResponse(c *gin.Context, resp gwdomain.RunResponse) {
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
			IntentID:   resp.IntentID,
			ApprovalID: resp.ApprovalID,
			RiskClass:  resp.RiskClass,
			LeaseID:    resp.LeaseID,
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
			IntentID:   resp.IntentID,
			ApprovalID: resp.ApprovalID,
			RiskClass:  resp.RiskClass,
			LeaseID:    resp.LeaseID,
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
		IntentID:   resp.IntentID,
		ApprovalID: resp.ApprovalID,
		RiskClass:  resp.RiskClass,
		LeaseID:    resp.LeaseID,
	})
}

func toIntentDTO(item gwdomain.ExecutionIntent) gwdto.IntentItem {
	return gwdto.IntentItem{
		ID:                   item.ID.String(),
		RequestID:            item.RequestID,
		ToolName:             item.ToolName,
		Actor:                item.Actor,
		Role:                 item.Role,
		Scopes:               append([]string{}, item.Scopes...),
		Input:                cloneMap(item.Input),
		Context:              cloneMap(item.Context),
		PolicyID:             uuidToStringPtr(item.PolicyID),
		RiskClass:            string(item.RiskClass),
		Reason:               item.Reason,
		ApprovalID:           uuidToStringPtr(item.ApprovalID),
		Status:               string(item.Status),
		PreflightStatus:      string(item.PreflightStatus),
		PreflightSummary:     cloneMap(item.PreflightSummary),
		PreflightArtifactSHA: item.PreflightArtifactSHA,
		PreflightCompletedAt: item.PreflightCompletedAt,
		ExpiresAt:            item.ExpiresAt,
		ApprovedAt:           item.ApprovedAt,
		ExecutedAt:           item.ExecutedAt,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}

func toPreflightReviewDTO(item gwdomain.PreflightReview) gwdto.PreflightReviewResponse {
	return gwdto.PreflightReviewResponse{
		IntentID:       item.IntentID.String(),
		ToolName:       item.ToolName,
		RiskClass:      string(item.RiskClass),
		Reason:         item.Reason,
		Status:         string(item.Status),
		Summary:        cloneMap(item.Summary),
		ArtifactSHA256: item.ArtifactSHA256,
		CompletedAt:    item.CompletedAt,
		ApprovalID:     uuidToStringPtr(item.ApprovalID),
		IntentStatus:   string(item.IntentStatus),
	}
}

func uuidToStringPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}
