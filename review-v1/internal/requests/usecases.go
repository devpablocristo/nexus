package requests

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	approvaldomain "github.com/devpablocristo/nexus/review-v1/internal/approvals/usecases/domain"
	auditdomain "github.com/devpablocristo/nexus/review-v1/internal/audit/usecases/domain"
	requestdomain "github.com/devpablocristo/nexus/review-v1/internal/requests/usecases/domain"
)

const DefaultApprovalTTL = time.Hour

// Ports definidos en el consumidor (usecases)
type policyLister interface {
	ListActive(ctx context.Context) ([]struct {
		ID           uuid.UUID
		Name         string
		ActionType   *string
		TargetSystem *string
		Expression   string
		Effect       string
		RiskOverride *string
	}, error)
}

type approvalCreator interface {
	Create(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error)
}

type Usecases struct {
	reqRepo     Repository
	policyRepo  PolicyLister
	approvalRepo approvalCreator
	idemStore   IdempotencyStore
	audit       AuditSink
	evaluator   *PolicyEvaluator
	riskConfig  RiskConfig
	ai          AIContextualizer
	approvalTTL time.Duration
}

// PolicyLister es el port mínimo que requests necesita de policies.
type PolicyLister interface {
	ListActive(ctx context.Context) ([]PolicyForEval, error)
}

// PolicyForEval contiene solo los campos necesarios para evaluación.
type PolicyForEval struct {
	ID           uuid.UUID
	Name         string
	ActionType   *string
	TargetSystem *string
	Expression   string
	Effect       string
	RiskOverride *string
}

func NewUsecases(
	reqRepo Repository,
	policyRepo PolicyLister,
	approvalRepo approvalCreator,
	idemStore IdempotencyStore,
	audit AuditSink,
	evaluator *PolicyEvaluator,
	riskConfig RiskConfig,
	ai AIContextualizer,
	approvalTTL time.Duration,
) *Usecases {
	if approvalTTL <= 0 {
		approvalTTL = DefaultApprovalTTL
	}
	return &Usecases{
		reqRepo:      reqRepo,
		policyRepo:   policyRepo,
		approvalRepo: approvalRepo,
		idemStore:    idemStore,
		audit:        audit,
		evaluator:    evaluator,
		riskConfig:   riskConfig,
		ai:           ai,
		approvalTTL:  approvalTTL,
	}
}

type SubmitInput struct {
	IdempotencyKey *string
	RequesterType  string
	RequesterID    string
	RequesterName  string
	ActionType     string
	TargetSystem   string
	TargetResource string
	Params         map[string]any
	Reason         string
	Context        string
}

type SubmitOutput struct {
	RequestID      uuid.UUID
	Decision       string
	RiskLevel      string
	DecisionReason string
	Status         string
	Approval       *struct {
		ID        uuid.UUID
		ExpiresAt time.Time
	}
	AISummary  string
	AIDegraded bool
}

func (u *Usecases) Submit(ctx context.Context, in SubmitInput) (SubmitOutput, error) {
	// Idempotencia: si ya existe, retornar respuesta cacheada
	if in.IdempotencyKey != nil && *in.IdempotencyKey != "" {
		if reqID, resp, ok := u.idemStore.Get(ctx, *in.IdempotencyKey); ok {
			return rebuildOutputFromCache(reqID, resp), nil
		}
	}

	now := time.Now().UTC()
	req := requestdomain.Request{
		ID:             uuid.New(),
		IdempotencyKey: in.IdempotencyKey,
		RequesterType:  requestdomain.RequesterType(in.RequesterType),
		RequesterID:    in.RequesterID,
		RequesterName:  in.RequesterName,
		ActionType:     in.ActionType,
		TargetSystem:   in.TargetSystem,
		TargetResource: in.TargetResource,
		Params:         in.Params,
		Reason:         in.Reason,
		Context:        in.Context,
		Status:         requestdomain.StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if req.RequesterType == "" {
		req.RequesterType = requestdomain.RequesterTypeAgent
	}
	if req.Params == nil {
		req.Params = make(map[string]any)
	}

	// Audit: best-effort, nunca falla la request
	logAuditError(
		u.audit.AppendEvent(ctx, req.ID, auditdomain.EventReceived, "requester", req.RequesterID,
			"Request received: "+req.ActionType+" "+req.TargetResource, nil),
		req.ID, auditdomain.EventReceived,
	)

	// Evaluar políticas
	policyList, err := u.policyRepo.ListActive(ctx)
	if err != nil {
		return SubmitOutput{}, fmt.Errorf("list active policies: %w", err)
	}

	requestMap := requestToMap(req)
	matched := false
	for _, p := range policyList {
		if (p.ActionType != nil && *p.ActionType != "" && *p.ActionType != req.ActionType) ||
			(p.TargetSystem != nil && *p.TargetSystem != "" && *p.TargetSystem != req.TargetSystem) {
			continue
		}
		match, evalErr := u.evaluator.Matches(p.Expression, requestMap, now)
		if evalErr != nil {
			slog.Error("policy evaluation error", "error", evalErr, "policy_id", p.ID, "expression", p.Expression)
			continue
		}
		if !match {
			continue
		}
		matched = true
		req.RiskLevel = TierRisk(req.ActionType, p.RiskOverride, u.riskConfig)
		dec, ok := DecideFromPolicy(p.Effect, req.RiskLevel)
		if !ok {
			continue
		}
		req.Decision = dec
		req.DecisionReason = "Policy '" + p.Name + "'"
		req.PolicyID = &p.ID
		break
	}
	if !matched {
		req.RiskLevel = TierRisk(req.ActionType, nil, u.riskConfig)
		req.Decision = DefaultDecision(req.RiskLevel)
		req.DecisionReason = "No policy matched; default for risk " + string(req.RiskLevel)
	}

	req.EvaluatedAt = &now
	req.Status = requestdomain.StatusEvaluated

	logAuditError(
		u.audit.AppendEvent(ctx, req.ID, auditdomain.EventEvaluated, "system", "nexus",
			"Risk: "+string(req.RiskLevel)+". "+req.DecisionReason+". Decision: "+string(req.Decision), nil),
		req.ID, auditdomain.EventEvaluated,
	)

	switch req.Decision {
	case requestdomain.DecisionAllow:
		return u.finalizeDecision(ctx, req, in.IdempotencyKey, now, auditdomain.EventAllowed, requestdomain.StatusAllowed)
	case requestdomain.DecisionDeny:
		return u.finalizeDecision(ctx, req, in.IdempotencyKey, now, auditdomain.EventDenied, requestdomain.StatusDenied)
	}

	// Require approval
	return u.handleRequireApproval(ctx, req, in, now)
}

func (u *Usecases) finalizeDecision(ctx context.Context, req requestdomain.Request, idemKey *string, now time.Time, auditEvent string, status requestdomain.RequestStatus) (SubmitOutput, error) {
	req.Status = status
	req.DecidedAt = &now
	req.UpdatedAt = now

	if _, err := u.reqRepo.Create(ctx, req); err != nil {
		return SubmitOutput{}, fmt.Errorf("create request: %w", err)
	}

	logAuditError(
		u.audit.AppendEvent(ctx, req.ID, auditEvent, "system", "nexus", "Request "+string(status), nil),
		req.ID, auditEvent,
	)

	out := SubmitOutput{
		RequestID:      req.ID,
		Decision:       string(req.Decision),
		RiskLevel:      string(req.RiskLevel),
		DecisionReason: req.DecisionReason,
		Status:         string(req.Status),
	}
	u.cacheIdempotency(ctx, idemKey, req.ID, out, now.Add(24*time.Hour))
	return out, nil
}

func (u *Usecases) handleRequireApproval(ctx context.Context, req requestdomain.Request, in SubmitInput, now time.Time) (SubmitOutput, error) {
	expiresAt := now.Add(u.approvalTTL)
	approval := approvaldomain.Approval{
		ID:        uuid.New(),
		RequestID: req.ID,
		Status:    approvaldomain.ApprovalStatusPending,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	approval, err := u.approvalRepo.Create(ctx, approval)
	if err != nil {
		return SubmitOutput{}, fmt.Errorf("create approval: %w", err)
	}
	req.ApprovalID = &approval.ID
	req.Status = requestdomain.StatusPendingApproval
	req.ExpiresAt = &expiresAt

	// AI: best-effort con fallback
	summary, degraded, aiErr := u.ai.Summarize(ctx, SummarizeInput{
		RequesterType: in.RequesterType, RequesterID: in.RequesterID, ActionType: in.ActionType,
		TargetSystem: in.TargetSystem, TargetResource: in.TargetResource, Params: in.Params,
		Reason: in.Reason, Context: in.Context,
		Decision: string(req.Decision), DecisionReason: req.DecisionReason, RiskLevel: string(req.RiskLevel),
	})
	if aiErr != nil {
		slog.Error("ai contextualizer failed", "error", aiErr, "request_id", req.ID)
	}
	req.AISummary = summary
	req.AIDegraded = degraded
	req.UpdatedAt = now

	if _, err := u.reqRepo.Create(ctx, req); err != nil {
		return SubmitOutput{}, fmt.Errorf("create request: %w", err)
	}

	logAuditError(
		u.audit.AppendEvent(ctx, req.ID, auditdomain.EventSentToApproval, "system", "nexus",
			"Sent to approval. Expires at "+expiresAt.Format(time.RFC3339), nil),
		req.ID, auditdomain.EventSentToApproval,
	)

	out := SubmitOutput{
		RequestID:      req.ID,
		Decision:       string(req.Decision),
		RiskLevel:      string(req.RiskLevel),
		DecisionReason: req.DecisionReason,
		Status:         string(req.Status),
		Approval: &struct {
			ID        uuid.UUID
			ExpiresAt time.Time
		}{ID: approval.ID, ExpiresAt: approval.ExpiresAt},
		AISummary:  summary,
		AIDegraded: degraded,
	}
	u.cacheIdempotency(ctx, in.IdempotencyKey, req.ID, out, expiresAt)
	return out, nil
}

func (u *Usecases) cacheIdempotency(ctx context.Context, idemKey *string, reqID uuid.UUID, out SubmitOutput, expiresAt time.Time) {
	if idemKey == nil || *idemKey == "" {
		return
	}
	resp := map[string]any{
		"request_id": reqID.String(), "decision": out.Decision,
		"risk_level": out.RiskLevel, "status": out.Status,
		"ai_summary": out.AISummary, "ai_degraded": out.AIDegraded,
	}
	if out.Approval != nil {
		resp["approval"] = map[string]any{
			"id":         out.Approval.ID.String(),
			"expires_at": out.Approval.ExpiresAt.Format(time.RFC3339),
		}
	}
	if err := u.idemStore.Set(ctx, *idemKey, reqID, resp, expiresAt); err != nil {
		slog.Error("idempotency store set failed", "error", err, "key", *idemKey)
	}
}

func rebuildOutputFromCache(reqID uuid.UUID, resp map[string]any) SubmitOutput {
	out := SubmitOutput{RequestID: reqID}
	if id, _ := resp["request_id"].(string); id != "" {
		out.RequestID, _ = uuid.Parse(id)
	}
	out.Decision, _ = resp["decision"].(string)
	out.RiskLevel, _ = resp["risk_level"].(string)
	out.Status, _ = resp["status"].(string)
	out.AISummary, _ = resp["ai_summary"].(string)
	out.AIDegraded, _ = resp["ai_degraded"].(bool)
	if a, ok := resp["approval"].(map[string]any); ok {
		out.Approval = &struct {
			ID        uuid.UUID
			ExpiresAt time.Time
		}{}
		if id, _ := a["id"].(string); id != "" {
			out.Approval.ID, _ = uuid.Parse(id)
		}
		if exp, _ := a["expires_at"].(string); exp != "" {
			out.Approval.ExpiresAt, _ = time.Parse(time.RFC3339, exp)
		}
	}
	return out
}

func requestToMap(r requestdomain.Request) map[string]any {
	params := r.Params
	if params == nil {
		params = make(map[string]any)
	}
	return map[string]any{
		"action_type":     r.ActionType,
		"target_system":   r.TargetSystem,
		"target_resource": r.TargetResource,
		"params":          params,
		"reason":          r.Reason,
		"context":         r.Context,
		"requester_type":  string(r.RequesterType),
		"requester_id":    r.RequesterID,
	}
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error) {
	return u.reqRepo.GetByID(ctx, id)
}

func (u *Usecases) List(ctx context.Context, status, actionType string, limit int) ([]requestdomain.Request, error) {
	return u.reqRepo.List(ctx, status, actionType, limit)
}

type ReportResultInput struct {
	Success      bool
	Result       map[string]any
	DurationMs   int64
	ErrorMessage string
}

func (u *Usecases) ReportResult(ctx context.Context, requestID uuid.UUID, in ReportResultInput) error {
	req, err := u.reqRepo.GetByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}
	now := time.Now().UTC()
	req.UpdatedAt = now
	req.ExecutedAt = &now
	if in.Success {
		req.Status = requestdomain.StatusExecuted
		req.ExecutionResult = in.Result
		logAuditError(
			u.audit.AppendEvent(ctx, requestID, auditdomain.EventExecuted, "requester", req.RequesterID,
				"Executed successfully", map[string]any{"result": in.Result, "duration_ms": in.DurationMs}),
			requestID, auditdomain.EventExecuted,
		)
	} else {
		req.Status = requestdomain.StatusFailed
		req.ErrorMessage = in.ErrorMessage
		logAuditError(
			u.audit.AppendEvent(ctx, requestID, auditdomain.EventExecutionFailed, "requester", req.RequesterID,
				in.ErrorMessage, nil),
			requestID, auditdomain.EventExecutionFailed,
		)
	}
	if _, err := u.reqRepo.Update(ctx, req); err != nil {
		return fmt.Errorf("update request: %w", err)
	}
	return nil
}

// json.Marshal check en compile-time
var _ = func() struct{} {
	var o SubmitOutput
	_, _ = json.Marshal(o)
	return struct{}{}
}
