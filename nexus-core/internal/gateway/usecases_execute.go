package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	auditdomain "nexus-core/internal/audit/usecases/domain"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus-core/pkg/types"
	"nexus-core/pkg/utils"
	"nexus-core/pkg/validations/jsonschema"
)

// validateOutputSchema valida el resultado contra output_schema; devuelve respuesta si falla.
func (s *service) validateOutputSchema(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (gwdomain.RunResponse, bool) {
	if st.execErr != nil || len(st.tool.OutputSchemaJSON) == 0 {
		return gwdomain.RunResponse{}, false
	}
	outSchema, err := s.cache.Compile(ctx, st.tool.ID.String()+":out", st.tool.OutputSchemaJSON)
	if err != nil || jsonschema.Validate(outSchema, st.result) != nil {
		code := types.ErrCodeOutputSchemaInvalid
		msg := "tool output does not match schema"
		s.auditRunCreate(ctx, orgID, req, st, auditdomain.StatusError, utils.Redact(st.result), &code, &msg)
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &gwdomain.RunResponse{Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, HTTPStatus: http.StatusBadGateway})
		s.observeRun(ctx, st.tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusError), time.Duration(st.latency)*time.Millisecond)
		return gwdomain.RunResponse{
			RequestID: st.requestID, Decision: gwdomain.DecisionAllow, ToolName: st.tool.Name,
			Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, LatencyMS: st.latency,
			HTTPStatus: http.StatusBadGateway, Idempotency: st.idemMeta,
		}, true
	}
	return gwdomain.RunResponse{}, false
}

// handleExecError audita y devuelve respuesta cuando hubo error de ejecución.
func (s *service) handleExecError(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (gwdomain.RunResponse, bool) {
	if st.execErr == nil {
		return gwdomain.RunResponse{}, false
	}
	code := st.execErr.Code
	if st.execErr.Code == types.ErrCodeTimeout && st.budget.RemainingMS() <= 0 {
		code = types.ErrCodeTimeoutBudget
	}
	msg := st.execErr.Message
	s.log.Error().
		Str("request_id", st.requestID).Str("org_id", orgID.String()).Str("tool_name", st.tool.Name).
		Str("decision", "allow").Str("status", "error").Str("error_code", code).
		Msg("run_error")
	s.auditRunCreate(ctx, orgID, req, st, auditdomain.StatusError, nil, &code, &msg)
	status := http.StatusBadGateway
	if st.execErr.Code == types.ErrCodeInvalidGETInput || st.execErr.Code == types.ErrCodeValidation {
		status = http.StatusBadRequest
	}
	if code == types.ErrCodeTimeoutBudget {
		status = http.StatusRequestTimeout
	}
	respForIdem := gwdomain.RunResponse{Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, HTTPStatus: status}
	s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &respForIdem)
	annotateRunSpan(ctx, orgID, st.input, st.tool.Name, st.requestID, "allow", st.policyID)
	s.observeRun(ctx, st.tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusError), time.Duration(st.latency)*time.Millisecond)
	return gwdomain.RunResponse{
		RequestID: st.requestID, Decision: gwdomain.DecisionAllow, ToolName: st.tool.Name,
		Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, LatencyMS: st.latency,
		HTTPStatus: status, Idempotency: st.idemMeta,
	}, true
}

// auditRunCreate escribe un evento de auditoría para el run.
func (s *service) auditRunCreate(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState, status auditdomain.Status, output any, errCode, errMsg *string) {
	ev := auditdomain.AuditEvent{
		OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
		Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
		InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
		DLPSummary: st.dlpSummary, Decision: auditdomain.DecisionAllow, PolicyID: st.policyID,
		Reason: strPtr(st.matchReason), Status: status, LatencyMS: int(st.latency),
		IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
		TimeoutMS: intPtr(req.TimeoutMS), BudgetRemainingMSAtExecute: intPtr(st.remainingBeforeExec),
		StageDurationsMS: st.budget.StageDurationsMS(),
	}
	if output != nil {
		ev.OutputRedacted = output
	}
	if errCode != nil {
		ev.ErrorCode = errCode
	}
	if errMsg != nil {
		ev.ErrorMessage = errMsg
	}
	if err := s.auditRepo.Create(ctx, ev); err != nil {
		s.log.Warn().Err(err).Str("request_id", st.requestID).Msg("audit_create_failed")
	}
}

// auditSuccessAndComplete audita éxito, marca idempotency completed y devuelve respuesta.
func (s *service) auditSuccessAndComplete(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState, tool tooldomain.Tool) gwdomain.RunResponse {
	s.auditRunCreate(ctx, orgID, req, st, auditdomain.StatusSuccess, utils.Redact(st.result), nil, nil)
	if st.createdIdempotencyRow {
		if err := s.idempotency.MarkCompleted(ctx, orgID, tool.Name, st.idempotencyKey, map[string]any{
			"decision": string(gwdomain.DecisionAllow), "status": string(gwdomain.RunStatusSuccess), "result": utils.Redact(st.result),
		}); err != nil {
			s.log.Warn().Err(err).Str("request_id", st.requestID).Str("tool_name", tool.Name).Msg("idempotency_mark_completed_failed")
		}
	}
	s.log.Info().
		Str("request_id", st.requestID).Str("org_id", orgID.String()).Str("tool_name", tool.Name).
		Str("decision", "allow").Str("status", "success").Int64("latency_ms", st.latency).
		Msg("run_success")
	annotateRunSpan(ctx, orgID, st.input, tool.Name, st.requestID, "allow", st.policyID)
	s.observeRun(ctx, tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusSuccess), time.Duration(st.latency)*time.Millisecond)
	return gwdomain.RunResponse{
		RequestID: st.requestID, Decision: gwdomain.DecisionAllow, ToolName: tool.Name,
		Status: gwdomain.RunStatusSuccess, Result: st.result, LatencyMS: st.latency,
		HTTPStatus: http.StatusOK, Idempotency: st.idemMeta,
	}
}
