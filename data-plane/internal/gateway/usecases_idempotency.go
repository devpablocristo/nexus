package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	auditdomain "data-plane/internal/audit/usecases/domain"
	gwdomain "data-plane/internal/gateway/usecases/domain"
	"nexus/pkg/types"
	"nexus/pkg/utils"
)

// handleIdempotencyReplayCompleted devuelve la respuesta cacheada cuando existe un registro completed.
func (u *Usecases) handleIdempotencyReplayCompleted(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState, existing *gwdomain.IdempotencyRecord) *gwdomain.RunResponse {
	st.idemMeta.Outcome = gwdomain.IdempotencyReplay
	latency := time.Since(st.start).Milliseconds()
	var result any
	status := gwdomain.RunStatusSuccess
	decision := gwdomain.DecisionAllow
	var reason *string
	var errCode *string
	var errMsg *string
	if existing.ResponseRedacted != nil {
		result = existing.ResponseRedacted["result"]
		if v, ok := existing.ResponseRedacted["status"].(string); ok && v != "" {
			status = gwdomain.RunStatus(v)
		}
		if v, ok := existing.ResponseRedacted["decision"].(string); ok && v != "" {
			decision = gwdomain.Decision(v)
		}
		if v, ok := existing.ResponseRedacted["reason"].(string); ok && v != "" {
			reason = &v
		}
		if errObj, ok := existing.ResponseRedacted["error"].(map[string]any); ok {
			if v, ok := errObj["code"].(string); ok && v != "" {
				errCode = &v
			}
			if v, ok := errObj["message"].(string); ok && v != "" {
				errMsg = &v
			}
		}
	}
	_ = u.auditRepo.Create(ctx, auditdomain.AuditEvent{
		OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
		Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
		InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
		DLPSummary: map[string]any{}, Decision: auditdomain.Decision(decision), Reason: reason,
		Status: auditdomain.Status(status), OutputRedacted: utils.Redact(result),
		ErrorCode: errCode, ErrorMessage: errMsg, LatencyMS: int(latency),
		IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
		TimeoutMS: intPtr(req.TimeoutMS),
	})
	u.observeRun(ctx, st.tool.Name, string(decision), string(status), time.Duration(latency)*time.Millisecond)
	httpStatus := http.StatusOK
	if status == gwdomain.RunStatusError {
		httpStatus = http.StatusBadGateway
	}
	if status == gwdomain.RunStatusBlocked {
		httpStatus = http.StatusForbidden
	}
	return &gwdomain.RunResponse{
		RequestID: st.requestID, Decision: decision, ToolName: st.tool.Name, Status: status,
		Reason: reason, Result: result, ErrorCode: errCode, ErrorMsg: errMsg,
		LatencyMS: latency, HTTPStatus: httpStatus, Idempotency: st.idemMeta,
	}
}

// handleIdempotencyReplayFailed devuelve la respuesta cuando existe un registro failed (replay del error).
func (u *Usecases) handleIdempotencyReplayFailed(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState, existing *gwdomain.IdempotencyRecord) *gwdomain.RunResponse {
	st.idemMeta.Outcome = gwdomain.IdempotencyReplay
	latency := time.Since(st.start).Milliseconds()
	code := types.ErrCodeInternal
	msg := "previous failed idempotent request"
	httpStatus := http.StatusBadGateway
	status := gwdomain.RunStatusError
	decision := gwdomain.DecisionAllow
	var reason *string
	if existing.ErrorCode != nil && *existing.ErrorCode != "" {
		code = *existing.ErrorCode
	}
	if existing.ResponseRedacted != nil {
		if v, ok := existing.ResponseRedacted["status"].(string); ok && v != "" {
			status = gwdomain.RunStatus(v)
		}
		if v, ok := existing.ResponseRedacted["decision"].(string); ok && v != "" {
			decision = gwdomain.Decision(v)
		}
		if v, ok := existing.ResponseRedacted["http_status"].(float64); ok && int(v) > 0 {
			httpStatus = int(v)
		}
		if errObj, ok := existing.ResponseRedacted["error"].(map[string]any); ok {
			if v, ok := errObj["code"].(string); ok && v != "" {
				code = v
			}
			if v, ok := errObj["message"].(string); ok && v != "" {
				msg = v
			}
		}
		if v, ok := existing.ResponseRedacted["reason"].(string); ok && v != "" {
			reason = &v
		}
	}
	errCode := code
	errMsg := msg
	_ = u.auditRepo.Create(ctx, auditdomain.AuditEvent{
		OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
		Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
		InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
		DLPSummary: map[string]any{}, Decision: auditdomain.Decision(decision), Reason: reason,
		Status: auditdomain.Status(status), ErrorCode: &errCode, ErrorMessage: &errMsg,
		LatencyMS: int(latency), IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
		TimeoutMS: intPtr(req.TimeoutMS),
	})
	u.observeRun(ctx, st.tool.Name, string(decision), string(status), time.Duration(latency)*time.Millisecond)
	return &gwdomain.RunResponse{
		RequestID: st.requestID, Decision: decision, ToolName: st.tool.Name, Status: status,
		Reason: reason, ErrorCode: &errCode, ErrorMsg: &errMsg, LatencyMS: latency,
		HTTPStatus: httpStatus, Idempotency: st.idemMeta,
	}
}
