package action

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

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
}

// NewUsecases builds action use cases.
func NewUsecases(repo Repository) *Usecases {
	return &Usecases{
		repo:            repo,
		executor:        NewDeterministicExecutor(),
		policyEvaluator: NewActionPolicyEvaluator(),
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

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (actiondomain.Action, error) {
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

	action, err := evaluateAction(nowUTC(), req, resource, policies, u.policyEvaluator)
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
	if created.Approval != nil {
		created.Approval.ActionID = created.ID
	}
	for idx := range created.Evidence {
		created.Evidence[idx].ActionID = created.ID
	}
	if created.Status == actiondomain.ActionStatusBlocked {
		u.emitIncident(ctx, created, IncidentTriggerBlockedAction, blockedIncidentReason(created), map[string]any{
			"decision":      string(created.Decision),
			"status":        string(created.Status),
			"source_system": created.SourceSystem,
		})
	}
	return created, nil
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
