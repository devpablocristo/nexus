package action

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
	"github.com/google/uuid"

	actionrisk "nexus/v2/data-plane/internal/action/risk"
	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type CreateRequest struct {
	ActionType    actiondomain.ActionType
	ResourceID    string
	ResourceType  actiondomain.ResourceType
	SourceSystem  string
	Justification string
	RequestedBy   actiondomain.ActorRef
	ProposedBy    actiondomain.ActorRef
	Payload       json.RawMessage
	Metadata      map[string]any
}

type ListRequest struct {
	ActionType string
	Status     string
	Limit      int
}

type DecideRequest struct {
	DecidedBy actiondomain.ActorRef
	Comment   string
}

type ExecuteRequest struct {
	LeaseID    uuid.UUID
	ExecutedBy actiondomain.ActorRef
}

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e httpError) Error() string {
	return e.Message
}

func newHTTPError(status int, code, message string) error {
	return httpError{Status: status, Code: code, Message: message}
}

// Usecases orchestrates the action aggregate.
type Usecases struct {
	repo            Repository
	executor        Executor
	resources       ResourceResolver
	policies        PolicySource
	policyEvaluator actionPolicyEvaluator
	incidents       IncidentSink
	audit           AuditSink
	metrics         MetricsSink
	riskContext     RiskContextProvider
}

// NewUsecases builds action use cases.
func NewUsecases(repo Repository) *Usecases {
	return &Usecases{
		repo:            repo,
		executor:        NewDeterministicExecutor(),
		policyEvaluator: NewActionPolicyEvaluator(),
		riskContext:     NewHistoricalRiskContextProvider(repo, NewInMemoryRiskBaselineStore()),
	}
}

func (u *Usecases) WithExecutor(executor Executor) *Usecases {
	if executor != nil {
		u.executor = executor
	}
	return u
}

func (u *Usecases) WithResourceResolver(resolver ResourceResolver) *Usecases {
	u.resources = resolver
	return u
}



func (u *Usecases) WithPolicySource(source PolicySource) *Usecases {
	u.policies = source
	return u
}

func (u *Usecases) WithRiskContextProvider(provider RiskContextProvider) *Usecases {
	u.riskContext = provider
	return u
}

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (actiondomain.Action, error) {
	ctx = WithDegradationCollector(ctx)
	if req.ActionType == "" {
		return actiondomain.Action{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "action_type required")
	}
	if req.ResourceID == "" {
		return actiondomain.Action{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "resource_id required")
	}
	if req.ResourceType == "" {
		return actiondomain.Action{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "resource_type required")
	}
	if req.SourceSystem == "" {
		return actiondomain.Action{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "source_system required")
	}
	if req.Justification == "" {
		return actiondomain.Action{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "justification required")
	}
	if err := validateActor("requested_by", req.RequestedBy); err != nil {
		return actiondomain.Action{}, err
	}
	if err := validateActor("proposed_by", req.ProposedBy); err != nil {
		return actiondomain.Action{}, err
	}
	if err := validateResourceType(req.ResourceType); err != nil {
		return actiondomain.Action{}, err
	}
	if err := validateActionType(req.ActionType); err != nil {
		return actiondomain.Action{}, err
	}

	resource, err := u.resolveResource(ctx, req)
	if err != nil {
		return actiondomain.Action{}, err
	}
	policies, err := u.listPolicies(ctx, req.ActionType, req.ResourceType)
	if err != nil {
		return actiondomain.Action{}, err
	}

	now := nowUTC()
	riskContext := actionrisk.Context{Now: now}
	if u.riskContext != nil {
		riskContext, err = u.riskContext.ContextFor(ctx, req, resource, now)
		if err != nil {
			return actiondomain.Action{}, err
		}
	}

	action, err := evaluateAction(now, req, resource, policies, u.policyEvaluator, riskContext)
	if err != nil {
		var httpErr httpError
		if errors.As(err, &httpErr) {
			return actiondomain.Action{}, httpErr
		}
		return actiondomain.Action{}, newHTTPError(http.StatusBadRequest, "VALIDATION", err.Error())
	}
	if action.Approval != nil {
		action.Approval.ActionID = action.ID
	}
	for idx := range action.Evidence {
		action.Evidence[idx].ActionID = action.ID
	}

	created, err := u.repo.Create(ctx, action)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if u.metrics != nil {
		u.metrics.IncActionCreated(string(created.Type))
	}
	if created.Approval != nil {
		created.Approval.ActionID = created.ID
	}
	for idx := range created.Evidence {
		created.Evidence[idx].ActionID = created.ID
	}
	auditData := map[string]any{
		"action_type": string(created.Type),
		"decision":    string(created.Decision),
		"status":      string(created.Status),
		"risk_level":  string(created.Risk.Level),
		"risk_score":  created.Risk.Score,
	}
	if d := DegradationFromContext(ctx); d != nil && d.IsDegraded() {
		auditData["degraded_context"] = true
	}
	u.emitAudit(ctx, sharedaudit.WriteRequest{
		EventType:     "action_created",
		SourceService: "data-plane",
		ActionID:      created.ID.String(),
		ResourceID:    created.ResourceID,
		ResourceType:  string(created.ResourceType),
		Actor:         actionAuditActor(created.ProposedBy),
		Summary:       "action created",
		Data:          auditData,
		OccurredAt:    created.CreatedAt,
	})
	if created.Status == actiondomain.ActionStatusBlocked {
		if u.metrics != nil {
			u.metrics.IncActionBlocked(string(created.Type))
		}
		trigger := IncidentTriggerBlockedAction
		reason := blockedIncidentReason(created)
		if actionMatchedTrapPolicy(created) {
			trigger = IncidentTriggerCanaryTriggered
			reason = canaryIncidentReason(created)
		}
		u.emitAudit(ctx, sharedaudit.WriteRequest{
			EventType:     "action_blocked",
			SourceService: "data-plane",
			ActionID:      created.ID.String(),
			ResourceID:    created.ResourceID,
			ResourceType:  string(created.ResourceType),
			Actor:         actionAuditActor(created.ProposedBy),
			Summary:       "action blocked",
			Data: map[string]any{
				"action_type": string(created.Type),
				"decision":    string(created.Decision),
				"status":      string(created.Status),
				"risk_level":  string(created.Risk.Level),
			},
			OccurredAt: created.UpdatedAt,
		})
		u.emitIncident(ctx, created, trigger, reason, map[string]any{
			"decision":      string(created.Decision),
			"status":        string(created.Status),
			"source_system": created.SourceSystem,
			"is_trap":       actionMatchedTrapPolicy(created),
		})
	}
	if u.riskContext != nil {
		if err := u.riskContext.ObserveAction(ctx, created); err != nil {
			sharedobservability.LoggerFromContext(ctx).Error(
				"risk context observe action failed",
				"action_id", created.ID.String(),
				"error", err,
			)
		}
	}
	return created, nil
}

func actionMatchedTrapPolicy(item actiondomain.Action) bool {
	for _, evidence := range item.Evidence {
		if evidence.Kind != "policy_decision" || evidence.Details == nil {
			continue
		}
		switch typed := evidence.Details["is_trap"].(type) {
		case bool:
			return typed
		case string:
			return strings.EqualFold(strings.TrimSpace(typed), "true")
		}
	}
	return false
}

func (u *Usecases) List(ctx context.Context, req ListRequest) ([]actiondomain.Action, error) {
	filters := ListFilters{
		ActionType: strings.TrimSpace(req.ActionType),
		Status:     strings.TrimSpace(req.Status),
		Limit:      req.Limit,
	}
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.ActionType != "" {
		if err := validateActionType(actiondomain.ActionType(filters.ActionType)); err != nil {
			return nil, err
		}
	}
	if filters.Status != "" {
		if err := validateStatus(filters.Status); err != nil {
			return nil, err
		}
	}
	return u.repo.List(ctx, filters)
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (actiondomain.Action, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return actiondomain.Action{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) GetRisk(ctx context.Context, id uuid.UUID) (actiondomain.RiskAssessment, error) {
	item, err := u.GetByID(ctx, id)
	if err != nil {
		return actiondomain.RiskAssessment{}, err
	}
	return item.Risk, nil
}

func (u *Usecases) GetEvidence(ctx context.Context, id uuid.UUID) ([]actiondomain.EvidenceRecord, error) {
	item, err := u.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return item.Evidence, nil
}

func (u *Usecases) Approve(ctx context.Context, id uuid.UUID, req DecideRequest) (actiondomain.Action, error) {
	if err := validateActor("decided_by", req.DecidedBy); err != nil {
		return actiondomain.Action{}, err
	}

	item, err := u.GetByID(ctx, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if err := ensureActionNotExpired(item, nowUTC()); err != nil {
		return actiondomain.Action{}, err
	}
	if item.Approval == nil {
		return actiondomain.Action{}, newHTTPError(http.StatusConflict, "APPROVAL_NOT_REQUIRED", "action does not require approval")
	}
	if !item.Approval.ExpiresAt.IsZero() && nowUTC().After(item.Approval.ExpiresAt) {
		return actiondomain.Action{}, newHTTPError(http.StatusForbidden, "APPROVAL_EXPIRED", "approval expired")
	}

	updated, err := u.repo.Decide(ctx, id, actiondomain.ApprovalStatusApproved, req.DecidedBy, strings.TrimSpace(req.Comment), nowUTC())
	if err != nil {
		return actiondomain.Action{}, mapRepoErr(err)
	}
	if u.metrics != nil {
		u.metrics.IncActionApproved(string(updated.Type))
	}
	u.emitAudit(ctx, sharedaudit.WriteRequest{
		EventType:     "action_approved",
		SourceService: "data-plane",
		ActionID:      updated.ID.String(),
		ResourceID:    updated.ResourceID,
		ResourceType:  string(updated.ResourceType),
		Actor:         actionAuditActor(req.DecidedBy),
		Summary:       "action approved",
		Data: map[string]any{
			"action_type": string(updated.Type),
			"decision":    string(updated.Decision),
			"status":      string(updated.Status),
			"comment":     strings.TrimSpace(req.Comment),
		},
		OccurredAt: updated.UpdatedAt,
	})
	return updated, nil
}

func (u *Usecases) Reject(ctx context.Context, id uuid.UUID, req DecideRequest) (actiondomain.Action, error) {
	if err := validateActor("decided_by", req.DecidedBy); err != nil {
		return actiondomain.Action{}, err
	}

	item, err := u.GetByID(ctx, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if err := ensureActionNotExpired(item, nowUTC()); err != nil {
		return actiondomain.Action{}, err
	}
	if item.Approval == nil {
		return actiondomain.Action{}, newHTTPError(http.StatusConflict, "APPROVAL_NOT_REQUIRED", "action does not require approval")
	}
	if !item.Approval.ExpiresAt.IsZero() && nowUTC().After(item.Approval.ExpiresAt) {
		return actiondomain.Action{}, newHTTPError(http.StatusForbidden, "APPROVAL_EXPIRED", "approval expired")
	}

	updated, err := u.repo.Decide(ctx, id, actiondomain.ApprovalStatusRejected, req.DecidedBy, strings.TrimSpace(req.Comment), nowUTC())
	if err != nil {
		return actiondomain.Action{}, mapRepoErr(err)
	}
	u.emitAudit(ctx, sharedaudit.WriteRequest{
		EventType:     "action_rejected",
		SourceService: "data-plane",
		ActionID:      updated.ID.String(),
		ResourceID:    updated.ResourceID,
		ResourceType:  string(updated.ResourceType),
		Actor:         actionAuditActor(req.DecidedBy),
		Summary:       "action rejected",
		Data: map[string]any{
			"action_type": string(updated.Type),
			"decision":    string(updated.Decision),
			"status":      string(updated.Status),
			"comment":     strings.TrimSpace(req.Comment),
		},
		OccurredAt: updated.UpdatedAt,
	})
	u.emitIncident(ctx, updated, IncidentTriggerApprovalRejected, rejectionIncidentReason(req.Comment), map[string]any{
		"decision":   string(updated.Decision),
		"status":     string(updated.Status),
		"decided_by": actorDetails(req.DecidedBy),
	})
	return updated, nil
}

func (u *Usecases) IssueLease(ctx context.Context, id uuid.UUID) (actiondomain.Action, error) {
	item, err := u.GetByID(ctx, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if err := ensureActionNotExpired(item, nowUTC()); err != nil {
		return actiondomain.Action{}, err
	}
	if item.Status != actiondomain.ActionStatusApproved {
		return actiondomain.Action{}, newHTTPError(http.StatusForbidden, "APPROVAL_REQUIRED", "action must be approved before issuing a lease")
	}
	if !allEvidencePassed(item.Evidence) {
		return actiondomain.Action{}, newHTTPError(http.StatusForbidden, "EVIDENCE_FAILED", "action evidence failed and cannot receive a lease")
	}

	lease := actiondomain.ExecutionLease{
		ID:        uuid.New(),
		ActionID:  item.ID,
		Status:    actiondomain.LeaseStatusActive,
		Scope:     actiondomain.LeaseScope{ActionID: item.ID, ActionType: item.Type, ResourceID: item.ResourceID, ResourceType: item.ResourceType},
		ExpiresAt: nowUTC().Add(leaseTTL(item.Risk.Level)),
		CreatedAt: nowUTC(),
	}

	updated, err := u.repo.IssueLease(ctx, id, lease)
	if err != nil {
		return actiondomain.Action{}, mapRepoErr(err)
	}
	u.emitAudit(ctx, sharedaudit.WriteRequest{
		EventType:     "action_leased",
		SourceService: "data-plane",
		ActionID:      updated.ID.String(),
		ResourceID:    updated.ResourceID,
		ResourceType:  string(updated.ResourceType),
		Summary:       "action lease issued",
		Data: map[string]any{
			"action_type": string(updated.Type),
			"status":      string(updated.Status),
			"lease_id":    updated.Lease.ID.String(),
			"expires_at":  updated.Lease.ExpiresAt,
		},
		OccurredAt: updated.UpdatedAt,
	})
	return updated, nil
}

func (u *Usecases) Execute(ctx context.Context, id uuid.UUID, req ExecuteRequest) (actiondomain.Action, error) {
	if req.LeaseID == uuid.Nil {
		return actiondomain.Action{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "lease_id required")
	}
	if err := validateActor("executed_by", req.ExecutedBy); err != nil {
		return actiondomain.Action{}, err
	}

	item, err := u.GetByID(ctx, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if err := ensureActionNotExpired(item, nowUTC()); err != nil {
		return actiondomain.Action{}, err
	}
	if item.Status != actiondomain.ActionStatusLeased {
		return actiondomain.Action{}, newHTTPError(http.StatusForbidden, "LEASE_REQUIRED", "active lease required before execution")
	}

	result, err := u.executor.Execute(ctx, item, req.ExecutedBy)
	if err != nil {
		u.emitAudit(ctx, sharedaudit.WriteRequest{
			EventType:     "action_execution_failed",
			SourceService: "data-plane",
			ActionID:      item.ID.String(),
			ResourceID:    item.ResourceID,
			ResourceType:  string(item.ResourceType),
			Actor:         actionAuditActor(req.ExecutedBy),
			Summary:       "action execution failed",
			Data: map[string]any{
				"action_type": string(item.Type),
				"lease_id":    req.LeaseID.String(),
				"error":       executionIncidentReason(err),
			},
			OccurredAt: nowUTC(),
		})
		u.emitIncident(ctx, item, IncidentTriggerExecutionFailed, executionIncidentReason(err), map[string]any{
			"lease_id":    req.LeaseID.String(),
			"executed_by": actorDetails(req.ExecutedBy),
		})
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return actiondomain.Action{}, newHTTPError(http.StatusRequestTimeout, "TIMEOUT", "action execution timed out")
		}
		return actiondomain.Action{}, newHTTPError(http.StatusBadGateway, "EXECUTION_FAILED", err.Error())
	}

	execution := actiondomain.ExecutionResult{
		Status:     "success",
		ExecutedBy: req.ExecutedBy,
		Result:     result,
		ExecutedAt: nowUTC(),
	}

	updated, err := u.repo.ConsumeLeaseAndMarkExecuted(ctx, id, req.LeaseID, execution)
	if err != nil {
		return actiondomain.Action{}, mapRepoErr(err)
	}
	if u.metrics != nil {
		u.metrics.IncActionExecuted(string(updated.Type))
	}
	u.emitAudit(ctx, sharedaudit.WriteRequest{
		EventType:     "action_executed",
		SourceService: "data-plane",
		ActionID:      updated.ID.String(),
		ResourceID:    updated.ResourceID,
		ResourceType:  string(updated.ResourceType),
		Actor:         actionAuditActor(req.ExecutedBy),
		Summary:       "action executed",
		Data: map[string]any{
			"action_type":  string(updated.Type),
			"status":       string(updated.Status),
			"lease_id":     req.LeaseID.String(),
			"execution_id": execution.Result["execution_id"],
		},
		OccurredAt: updated.Execution.ExecutedAt,
	})
	return updated, nil
}

func validateActionType(value actiondomain.ActionType) error {
	switch value {
	case actiondomain.ActionTypeWithdrawal, actiondomain.ActionTypeTreasuryTransfer, actiondomain.ActionTypeHotToColdMove:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported action_type")
	}
}

func validateResourceType(value actiondomain.ResourceType) error {
	switch value {
	case actiondomain.ResourceTypeWallet, actiondomain.ResourceTypeTreasury, actiondomain.ResourceTypeVault:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported resource_type")
	}
}

func validateStatus(value string) error {
	switch actiondomain.ActionStatus(value) {
	case actiondomain.ActionStatusPending, actiondomain.ActionStatusBlocked, actiondomain.ActionStatusPendingApproval, actiondomain.ActionStatusApproved, actiondomain.ActionStatusLeased, actiondomain.ActionStatusExecuted, actiondomain.ActionStatusRejected, actiondomain.ActionStatusExpired:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported status")
	}
}

func validateActor(field string, actor actiondomain.ActorRef) error {
	if strings.TrimSpace(actor.ID) == "" {
		return newHTTPError(http.StatusBadRequest, "VALIDATION", field+" id required")
	}
	switch actor.Type {
	case actiondomain.ActorTypeUser, actiondomain.ActorTypeSystem, actiondomain.ActorTypeAgent:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", field+" type must be user, system or agent")
	}
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "action not found")
	}
	if errors.Is(err, ErrApprovalNotPending) {
		return newHTTPError(http.StatusConflict, "ALREADY_DECIDED", "action approval already decided")
	}
	if errors.Is(err, ErrLeaseAlreadyIssued) {
		return newHTTPError(http.StatusConflict, "LEASE_ALREADY_ISSUED", "action already has an active or consumed lease")
	}
	if errors.Is(err, ErrLeaseNotFound) {
		return newHTTPError(http.StatusForbidden, "LEASE_REQUIRED", "active lease required before execution")
	}
	if errors.Is(err, ErrLeaseMismatch) || errors.Is(err, ErrLeaseNotActive) {
		return newHTTPError(http.StatusForbidden, "LEASE_INVALID", "execution lease is not active for this action")
	}
	if errors.Is(err, ErrLeaseExpired) {
		return newHTTPError(http.StatusForbidden, "LEASE_EXPIRED", "execution lease expired before execution")
	}
	if errors.Is(err, ErrActionAlreadyExecuted) {
		return newHTTPError(http.StatusConflict, "ACTION_ALREADY_EXECUTED", "action already executed")
	}
	return err
}

func (u *Usecases) resolveResource(ctx context.Context, req CreateRequest) (actiondomain.ProtectedResource, error) {
	if u.resources == nil {
		return actiondomain.ProtectedResource{
			ID:   strings.TrimSpace(req.ResourceID),
			Type: req.ResourceType,
		}, nil
	}

	resource, err := u.resources.GetByID(ctx, strings.TrimSpace(req.ResourceID))
	if err != nil {
		if errors.Is(err, ErrResourceNotFound) {
			return actiondomain.ProtectedResource{}, newHTTPError(http.StatusNotFound, "RESOURCE_NOT_FOUND", "resource not found")
		}
		return actiondomain.ProtectedResource{}, err
	}
	if resource.Type != req.ResourceType {
		return actiondomain.ProtectedResource{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "resource_type does not match resolved resource")
	}
	return resource, nil
}

func (u *Usecases) listPolicies(ctx context.Context, actionType actiondomain.ActionType, resourceType actiondomain.ResourceType) ([]ActionPolicy, error) {
	if u.policies == nil {
		return nil, nil
	}
	return u.policies.List(ctx, string(actionType), string(resourceType))
}

func ensureActionNotExpired(item actiondomain.Action, now time.Time) error {
	if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
		return newHTTPError(http.StatusForbidden, "ACTION_EXPIRED", "action expired")
	}
	return nil
}

func allEvidencePassed(items []actiondomain.EvidenceRecord) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Status != actiondomain.EvidenceStatusPassed {
			return false
		}
	}
	return true
}

func leaseTTL(level actiondomain.RiskLevel) time.Duration {
	switch level {
	case actiondomain.RiskLevelCritical:
		return 2 * time.Minute
	case actiondomain.RiskLevelHigh:
		return 5 * time.Minute
	default:
		return 10 * time.Minute
	}
}
