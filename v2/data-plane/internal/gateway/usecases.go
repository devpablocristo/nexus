package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v5"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
	"nexus/v2/data-plane/internal/policy"
	policydomain "nexus/v2/data-plane/internal/policy/usecases/domain"
	secretdomain "nexus/v2/data-plane/internal/secrets/usecases/domain"
	"nexus/v2/data-plane/internal/tool"
)

var (
	ErrToolNotFound        = errors.New("tool not found")
	ErrToolDisabled        = errors.New("tool disabled")
	ErrUnsupportedToolKind = errors.New("unsupported tool kind")
	ErrInputSchemaInvalid  = errors.New("input does not match schema")
	ErrOutputSchemaInvalid = errors.New("output does not match schema")
	ErrTimeoutExceeded     = errors.New("run timeout exceeded")
	ErrPolicyDecision      = errors.New("policy decision failed")
)

const (
	defaultTimeoutMS = 10000
	minTimeoutMS     = 1000
	maxTimeoutMS     = 30000
)

type ToolRepository interface {
	GetByID(ctx context.Context, id string) (tool.Definition, error)
	GetByName(ctx context.Context, name string) (tool.Definition, error)
}

type PolicyRepository interface {
	ListByToolName(ctx context.Context, toolName string) ([]policydomain.Policy, error)
}

type RateLimiter interface {
	Allow(key string, perMinute int) bool
}

type EgressChecker interface {
	IsHostAllowed(ctx context.Context, toolID, host string) (bool, error)
}

type SecretRepository interface {
	ListForTool(ctx context.Context, toolID string) ([]secretdomain.ToolSecret, error)
}

type ApprovalPort interface {
	RequestApproval(ctx context.Context, req ApprovalRequest) (string, error)
}

type IntentRepository interface {
	Create(ctx context.Context, intent gwdomain.ExecutionIntent) (gwdomain.ExecutionIntent, error)
	LinkApproval(ctx context.Context, intentID, approvalID uuid.UUID) error
	GetByID(ctx context.Context, intentID uuid.UUID) (gwdomain.ExecutionIntent, error)
	ListRecent(ctx context.Context, limit int) ([]gwdomain.ExecutionIntent, error)
	MarkExecuted(ctx context.Context, intentID uuid.UUID) error
}

type LeaseRepository interface {
	Create(ctx context.Context, lease gwdomain.ExecutionLease) (gwdomain.ExecutionLease, error)
	GetByID(ctx context.Context, leaseID uuid.UUID) (gwdomain.ExecutionLease, error)
	Consume(ctx context.Context, leaseID, intentID uuid.UUID) (gwdomain.ExecutionLease, error)
}

type Executor interface {
	Execute(ctx context.Context, method, rawURL string, input map[string]any, headers map[string]string) (any, error)
}

type ApprovalRequest struct {
	IntentID   *uuid.UUID
	RequestID  string
	ToolName   string
	Reason     string
	TTLSeconds int
}

type Usecases struct {
	tools       ToolRepository
	policies    PolicyRepository
	idempotency idempotencyRepository
	limiter     RateLimiter
	egress      EgressChecker
	secrets     SecretRepository
	approval    ApprovalPort
	intents     IntentRepository
	leases      LeaseRepository
	evaluator   *policy.Evaluator
	executor    Executor
}

type runState struct {
	start                 time.Time
	requestID             string
	intentID              string
	input                 map[string]any
	context               map[string]any
	tool                  tool.Definition
	result                any
	idemMeta              gwdomain.IdempotencyMeta
	idempotencyKey        string
	requestFingerprint    string
	createdIdempotencyRow bool
	headers               map[string]string
}

func NewUsecases(tools ToolRepository, policies PolicyRepository, idempotency idempotencyRepository, limiter RateLimiter, egress EgressChecker, secrets SecretRepository, evaluator *policy.Evaluator, executor Executor) *Usecases {
	return &Usecases{
		tools:       tools,
		policies:    policies,
		idempotency: idempotency,
		limiter:     limiter,
		egress:      egress,
		secrets:     secrets,
		evaluator:   evaluator,
		executor:    executor,
	}
}

func (u *Usecases) WithApproval(port ApprovalPort) *Usecases {
	u.approval = port
	return u
}

func (u *Usecases) WithIntentRepository(repo IntentRepository) *Usecases {
	u.intents = repo
	return u
}

func (u *Usecases) WithLeaseRepository(repo LeaseRepository) *Usecases {
	u.leases = repo
	return u
}

func (u *Usecases) GetIntent(ctx context.Context, intentID uuid.UUID) (gwdomain.ExecutionIntent, error) {
	if u.intents == nil {
		return gwdomain.ExecutionIntent{}, newRunHTTPError(http.StatusInternalServerError, "INTENTS_NOT_CONFIGURED", "intents are not configured", nil)
	}
	item, err := u.intents.GetByID(ctx, intentID)
	if err != nil {
		if errors.Is(err, ErrIntentNotFound) {
			return gwdomain.ExecutionIntent{}, newRunHTTPError(http.StatusNotFound, "NOT_FOUND", "intent not found", nil)
		}
		return gwdomain.ExecutionIntent{}, err
	}
	return item, nil
}

func (u *Usecases) GetIntentPreflight(ctx context.Context, intentID uuid.UUID) (gwdomain.PreflightReview, error) {
	item, err := u.GetIntent(ctx, intentID)
	if err != nil {
		return gwdomain.PreflightReview{}, err
	}
	return gwdomain.PreflightReview{
		IntentID:     item.ID,
		ToolName:     item.ToolName,
		RiskClass:    item.RiskClass,
		Reason:       item.Reason,
		Status:       item.PreflightStatus,
		Summary:      cloneMap(item.PreflightSummary),
		CompletedAt:  item.PreflightCompletedAt,
		ApprovalID:   item.ApprovalID,
		IntentStatus: item.Status,
	}, nil
}

func (u *Usecases) ListIntents(ctx context.Context, limit int) ([]gwdomain.ExecutionIntent, error) {
	if u.intents == nil {
		return nil, newRunHTTPError(http.StatusInternalServerError, "INTENTS_NOT_CONFIGURED", "intents are not configured", nil)
	}
	if limit <= 0 {
		limit = 50
	}
	return u.intents.ListRecent(ctx, limit)
}

func (u *Usecases) IssueExecutionLease(ctx context.Context, intentID uuid.UUID) (gwdomain.ExecutionLease, error) {
	if u.intents == nil || u.leases == nil {
		return gwdomain.ExecutionLease{}, newRunHTTPError(http.StatusInternalServerError, "LEASES_NOT_CONFIGURED", "execution leases are not configured", nil)
	}

	intent, err := u.intents.GetByID(ctx, intentID)
	if err != nil {
		if errors.Is(err, ErrIntentNotFound) {
			return gwdomain.ExecutionLease{}, newRunHTTPError(http.StatusNotFound, "NOT_FOUND", "intent not found", nil)
		}
		return gwdomain.ExecutionLease{}, err
	}
	if intent.Status != gwdomain.IntentStatusApproved {
		return gwdomain.ExecutionLease{}, newRunHTTPError(http.StatusForbidden, "APPROVAL_REQUIRED", "intent must be approved before issuing a lease", nil)
	}
	if !intent.ExpiresAt.IsZero() && time.Now().UTC().After(intent.ExpiresAt) {
		return gwdomain.ExecutionLease{}, newRunHTTPError(http.StatusForbidden, "LEASE_EXPIRED", "intent expired before lease issuance", nil)
	}
	if intent.PreflightStatus == gwdomain.PreflightStatusFailed {
		return gwdomain.ExecutionLease{}, newRunHTTPError(http.StatusForbidden, "PREFLIGHT_FAILED", "intent preflight failed and cannot receive a lease", nil)
	}

	return u.leases.Create(ctx, gwdomain.ExecutionLease{
		IntentID:        intent.ID,
		ToolName:        intent.ToolName,
		RiskClass:       intent.RiskClass,
		Status:          gwdomain.ExecutionLeaseStatusActive,
		CredentialMode:  executionLeaseCredentialMode(intent),
		CredentialHints: executionLeaseCredentialHints(intent),
		ExpiresAt:       time.Now().UTC().Add(time.Duration(executionLeaseTTLSeconds(intent.RiskClass)) * time.Second),
	})
}

func (u *Usecases) ExecuteIntent(ctx context.Context, intentID uuid.UUID, timeoutMS int) (gwdomain.RunResponse, error) {
	return u.ExecuteIntentWithLease(ctx, intentID, uuid.Nil, timeoutMS)
}

func (u *Usecases) ExecuteIntentWithLease(ctx context.Context, intentID, leaseID uuid.UUID, timeoutMS int) (gwdomain.RunResponse, error) {
	if u.intents == nil {
		return gwdomain.RunResponse{}, newRunHTTPError(http.StatusInternalServerError, "INTENTS_NOT_CONFIGURED", "intents are not configured", nil)
	}

	intent, err := u.intents.GetByID(ctx, intentID)
	if err != nil {
		if errors.Is(err, ErrIntentNotFound) {
			return gwdomain.RunResponse{}, newRunHTTPError(http.StatusNotFound, "NOT_FOUND", "intent not found", nil)
		}
		return gwdomain.RunResponse{}, err
	}

	if intent.Status != gwdomain.IntentStatusApproved {
		return gwdomain.RunResponse{
			RequestID:  newRequestID(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     "intent is not approved for execution",
			HTTPStatus: http.StatusForbidden,
			IntentID:   intent.ID.String(),
			ApprovalID: uuidStringValue(intent.ApprovalID),
		}, nil
	}
	if !intent.ExpiresAt.IsZero() && time.Now().UTC().After(intent.ExpiresAt) {
		return gwdomain.RunResponse{
			RequestID:  newRequestID(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     "intent expired before execution",
			HTTPStatus: http.StatusForbidden,
			IntentID:   intent.ID.String(),
			ApprovalID: uuidStringValue(intent.ApprovalID),
		}, nil
	}
	if intent.PreflightStatus == gwdomain.PreflightStatusFailed {
		return gwdomain.RunResponse{
			RequestID:  newRequestID(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     "intent preflight failed and cannot be executed",
			HTTPStatus: http.StatusForbidden,
			IntentID:   intent.ID.String(),
			ApprovalID: uuidStringValue(intent.ApprovalID),
		}, nil
	}
	if u.leases == nil || leaseID == uuid.Nil {
		return gwdomain.RunResponse{
			RequestID:  newRequestID(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     "execution lease required before executing intent",
			HTTPStatus: http.StatusForbidden,
			IntentID:   intent.ID.String(),
			ApprovalID: uuidStringValue(intent.ApprovalID),
		}, nil
	}

	lease, err := u.leases.Consume(ctx, leaseID, intent.ID)
	if err != nil {
		if errors.Is(err, ErrLeaseNotFound) {
			return gwdomain.RunResponse{
				RequestID:  newRequestID(),
				Decision:   gwdomain.DecisionDeny,
				ToolName:   intent.ToolName,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     "execution lease not found",
				HTTPStatus: http.StatusForbidden,
				IntentID:   intent.ID.String(),
				ApprovalID: uuidStringValue(intent.ApprovalID),
			}, nil
		}
		if errors.Is(err, ErrLeaseIntentMismatch) || errors.Is(err, ErrLeaseNotActive) {
			return gwdomain.RunResponse{
				RequestID:  newRequestID(),
				Decision:   gwdomain.DecisionDeny,
				ToolName:   intent.ToolName,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     "execution lease is not active for this intent",
				HTTPStatus: http.StatusForbidden,
				IntentID:   intent.ID.String(),
				ApprovalID: uuidStringValue(intent.ApprovalID),
			}, nil
		}
		if errors.Is(err, ErrLeaseExpired) {
			return gwdomain.RunResponse{
				RequestID:  newRequestID(),
				Decision:   gwdomain.DecisionDeny,
				ToolName:   intent.ToolName,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     "execution lease expired before execution",
				HTTPStatus: http.StatusForbidden,
				IntentID:   intent.ID.String(),
				ApprovalID: uuidStringValue(intent.ApprovalID),
			}, nil
		}
		return gwdomain.RunResponse{}, err
	}
	if lease.IntentID != intent.ID {
		return gwdomain.RunResponse{
			RequestID:  newRequestID(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     "execution lease is not active for this intent",
			HTTPStatus: http.StatusForbidden,
			IntentID:   intent.ID.String(),
			ApprovalID: uuidStringValue(intent.ApprovalID),
		}, nil
	}

	contextMap := cloneMap(intent.Context)
	contextMap["intent_id"] = intent.ID.String()
	contextMap["execution_lease"] = map[string]any{
		"lease_id":         lease.ID.String(),
		"credential_mode":  lease.CredentialMode,
		"credential_hints": cloneMap(lease.CredentialHints),
		"expires_at":       lease.ExpiresAt.UTC().Format(time.RFC3339),
	}

	resp, err := u.Run(ctx, gwdomain.RunRequest{
		RequestID: newRequestID(),
		ToolName:  intent.ToolName,
		ToolID:    intent.ToolID,
		IntentID:  intent.ID.String(),
		TimeoutMS: timeoutMS,
		Input:     cloneMap(intent.Input),
		Context:   contextMap,
	})
	if err != nil {
		return resp, err
	}

	resp.IntentID = intent.ID.String()
	resp.ApprovalID = uuidStringValue(intent.ApprovalID)
	if resp.Status == gwdomain.RunStatusSuccess || resp.Status == gwdomain.RunStatusError {
		_ = u.intents.MarkExecuted(ctx, intent.ID)
	}
	return resp, nil
}

func (u *Usecases) Run(ctx context.Context, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	st := runState{
		start:     time.Now(),
		requestID: req.RequestID,
		intentID:  req.IntentID,
		input:     req.Input,
		context:   req.Context,
	}
	if st.requestID == "" {
		st.requestID = newRequestID()
	}
	if st.input == nil {
		st.input = map[string]any{}
	}
	if st.context == nil {
		st.context = map[string]any{}
	}
	st.idemMeta = gwdomain.IdempotencyMeta{Present: req.IdempotencyKey != nil}
	if req.IdempotencyKey != nil {
		st.idempotencyKey = *req.IdempotencyKey
	}

	timeoutMS := clampTimeoutMS(req.TimeoutMS)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	selectedTool, err := u.resolveTool(runCtx, req.ToolID, req.ToolName)
	if err != nil {
		return gwdomain.RunResponse{}, toRunHTTPError(mapRunError(err), &st.idemMeta)
	}
	st.tool = selectedTool

	if resp, err := u.resolveIdempotency(runCtx, &st); err != nil {
		return gwdomain.RunResponse{}, toRunHTTPError(mapRunError(err), &st.idemMeta)
	} else if resp != nil {
		return *resp, nil
	}

	if err := u.validateAndPrepare(st); err != nil {
		mapped := toRunHTTPError(mapRunError(err), &st.idemMeta)
		if snapshot, ok := snapshotFromRunError(mapped); ok {
			u.markFailedIdempotency(runCtx, st, snapshot)
		}
		return gwdomain.RunResponse{}, mapped
	}
	if resp, err := u.decide(runCtx, st); err != nil {
		mapped := toRunHTTPError(mapRunError(err), &st.idemMeta)
		if snapshot, ok := snapshotFromRunError(mapped); ok {
			u.markFailedIdempotency(runCtx, st, snapshot)
		}
		return gwdomain.RunResponse{}, mapped
	} else if resp != nil {
		resp.Idempotency = st.idemMeta
		u.markFailedIdempotency(runCtx, st, snapshotFromRunResponse(*resp))
		return *resp, nil
	}
	if resp, err := u.prepareExecution(runCtx, &st); err != nil {
		mapped := toRunHTTPError(mapRunError(err), &st.idemMeta)
		if snapshot, ok := snapshotFromRunError(mapped); ok {
			u.markFailedIdempotency(runCtx, st, snapshot)
		}
		return gwdomain.RunResponse{}, mapped
	} else if resp != nil {
		resp.Idempotency = st.idemMeta
		u.markFailedIdempotency(runCtx, st, snapshotFromRunResponse(*resp))
		return *resp, nil
	}
	if err := u.executeAndFinish(runCtx, &st); err != nil {
		mapped := toRunHTTPError(mapRunError(err), &st.idemMeta)
		if snapshot, ok := snapshotFromRunError(mapped); ok {
			u.markFailedIdempotency(runCtx, st, snapshot)
		}
		return gwdomain.RunResponse{}, mapped
	}

	resp := gwdomain.RunResponse{
		RequestID:   st.requestID,
		Decision:    gwdomain.DecisionAllow,
		ToolName:    st.tool.Name,
		Status:      gwdomain.RunStatusSuccess,
		Result:      st.result,
		LatencyMS:   time.Since(st.start).Milliseconds(),
		HTTPStatus:  200,
		Idempotency: st.idemMeta,
	}
	if resp.Idempotency.Present && resp.Idempotency.Outcome == "" && strings.EqualFold(st.tool.Method, http.MethodGet) {
		resp.Idempotency.Outcome = gwdomain.IdempotencySkippedNotWrite
	}
	u.markCompletedIdempotency(runCtx, st, resp)
	return resp, nil
}

func (u *Usecases) resolveTool(ctx context.Context, toolID, toolName string) (tool.Definition, error) {
	var (
		selectedTool tool.Definition
		err          error
	)

	if toolID != "" {
		selectedTool, err = u.tools.GetByID(ctx, toolID)
	} else {
		selectedTool, err = u.tools.GetByName(ctx, toolName)
	}
	if err != nil {
		if errors.Is(err, tool.ErrNotFound) {
			return tool.Definition{}, ErrToolNotFound
		}
		return tool.Definition{}, err
	}
	return selectedTool, nil
}

func (u *Usecases) validateAndPrepare(st runState) error {
	if !st.tool.Enabled {
		return ErrToolDisabled
	}
	if st.tool.Kind != tool.KindHTTP {
		return ErrUnsupportedToolKind
	}
	if err := validateSchema(st.tool.InputSchemaJSON, st.input); err != nil {
		return ErrInputSchemaInvalid
	}
	return nil
}

func (u *Usecases) decide(ctx context.Context, st runState) (*gwdomain.RunResponse, error) {
	if u.policies == nil || u.evaluator == nil {
		return nil, nil
	}

	policies, err := u.policies.ListByToolName(ctx, st.tool.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPolicyDecision, err)
	}

	for _, current := range policies {
		matched, err := u.evaluator.Matches(current.Expression, st.input, st.context, policy.ToolAttributes{
			Name:   st.tool.Name,
			Kind:   string(st.tool.Kind),
			Method: st.tool.Method,
			URL:    st.tool.URL,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPolicyDecision, err)
		}
		if !matched {
			continue
		}
		if current.Effect == policydomain.EffectDeny {
			reason := current.Reason
			if reason == "" {
				reason = "blocked by policy"
			}

			resp := gwdomain.RunResponse{
				RequestID:  st.requestID,
				Decision:   gwdomain.DecisionDeny,
				ToolName:   st.tool.Name,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     reason,
				LatencyMS:  time.Since(st.start).Milliseconds(),
				HTTPStatus: 403,
			}
			return &resp, nil
		}
		if !current.RequireApproval {
			return nil, nil
		}
		if st.intentID != "" {
			return nil, nil
		}
		if u.approval == nil || u.intents == nil {
			return nil, newRunHTTPError(http.StatusInternalServerError, "APPROVAL_NOT_CONFIGURED", "approval flow not configured", &st.idemMeta)
		}

		reason := current.Reason
		if reason == "" {
			reason = "approval required"
		}
		riskClass := classifyRiskClass(st.tool, st.input, st.context)
		preflight := evaluateDeterministicPreflight(riskClass, st.input, st.context)
		if preflight.required && preflight.status == gwdomain.PreflightStatusFailed {
			resp := gwdomain.RunResponse{
				RequestID:  st.requestID,
				Decision:   gwdomain.DecisionDeny,
				ToolName:   st.tool.Name,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     preflight.failureReason,
				LatencyMS:  time.Since(st.start).Milliseconds(),
				HTTPStatus: preflight.failureHTTP,
			}
			return &resp, nil
		}

		intent, err := u.intents.Create(ctx, gwdomain.ExecutionIntent{
			ToolID:               st.tool.ID,
			ToolName:             st.tool.Name,
			RequestID:            st.requestID,
			Input:                cloneMap(st.input),
			Context:              cloneMap(st.context),
			PolicyID:             &current.ID,
			RiskClass:            riskClass,
			Reason:               reason,
			Status:               gwdomain.IntentStatusPendingApproval,
			PreflightStatus:      preflight.status,
			PreflightSummary:     cloneMap(preflight.summary),
			PreflightCompletedAt: nowPtrIfRequired(preflight.required),
			ExpiresAt:            time.Now().UTC().Add(time.Duration(current.ApprovalTTLSeconds) * time.Second),
		})
		if err != nil {
			return nil, err
		}

		approvalID, err := u.approval.RequestApproval(ctx, ApprovalRequest{
			IntentID:   &intent.ID,
			RequestID:  st.requestID,
			ToolName:   st.tool.Name,
			Reason:     reason,
			TTLSeconds: current.ApprovalTTLSeconds,
		})
		if err != nil {
			return nil, err
		}
		if parsedApprovalID, parseErr := uuid.Parse(approvalID); parseErr == nil {
			_ = u.intents.LinkApproval(ctx, intent.ID, parsedApprovalID)
		}

		resp := gwdomain.RunResponse{
			RequestID:  st.requestID,
			Decision:   gwdomain.DecisionDeny,
			ToolName:   st.tool.Name,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     "pending human approval (id: " + approvalID + ")",
			LatencyMS:  time.Since(st.start).Milliseconds(),
			HTTPStatus: http.StatusAccepted,
			IntentID:   intent.ID.String(),
			ApprovalID: approvalID,
		}
		return &resp, nil
	}

	return nil, nil
}

func (u *Usecases) executeAndFinish(ctx context.Context, st *runState) error {
	result, err := u.executor.Execute(ctx, st.tool.Method, st.tool.URL, st.input, st.headers)
	if err != nil {
		return err
	}
	if err := validateSchema(st.tool.OutputSchemaJSON, result); err != nil {
		return ErrOutputSchemaInvalid
	}
	st.result = result
	return nil
}

func (u *Usecases) prepareExecution(ctx context.Context, st *runState) (*gwdomain.RunResponse, error) {
	if u.limiter != nil && st.tool.RateLimitPerMinute > 0 {
		key := "tool:" + st.tool.ID
		if !u.limiter.Allow(key, st.tool.RateLimitPerMinute) {
			return &gwdomain.RunResponse{
				RequestID:  st.requestID,
				Decision:   gwdomain.DecisionDeny,
				ToolName:   st.tool.Name,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     "rate limit exceeded",
				LatencyMS:  timeSinceMS(st.start),
				HTTPStatus: http.StatusForbidden,
			}, nil
		}
	}

	parsed, err := url.Parse(st.tool.URL)
	if err != nil || parsed.Hostname() == "" {
		return nil, newRunHTTPError(http.StatusBadRequest, "INVALID_TOOL_URL", "invalid tool url", &st.idemMeta)
	}
	if u.egress != nil {
		allowed, err := u.egress.IsHostAllowed(ctx, st.tool.ID, parsed.Hostname())
		if err != nil {
			return nil, newRunHTTPError(http.StatusInternalServerError, "EGRESS_STORE_ERROR", "egress store read failed", &st.idemMeta)
		}
		if !allowed {
			return &gwdomain.RunResponse{
				RequestID:  st.requestID,
				Decision:   gwdomain.DecisionDeny,
				ToolName:   st.tool.Name,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     "egress host denied",
				LatencyMS:  timeSinceMS(st.start),
				HTTPStatus: http.StatusForbidden,
			}, nil
		}
	}

	st.headers = map[string]string{
		"X-Nexus-Request-Id": st.requestID,
	}
	if u.secrets == nil {
		return nil, nil
	}
	secrets, err := u.secrets.ListForTool(ctx, st.tool.ID)
	if err != nil {
		return nil, newRunHTTPError(http.StatusInternalServerError, "SECRETS_STORE_ERROR", "secrets store read failed", &st.idemMeta)
	}
	for _, secret := range secrets {
		if !secret.Enabled {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(secret.SecretType)) {
		case "header":
			if strings.TrimSpace(secret.KeyName) != "" {
				st.headers[secret.KeyName] = secret.PlaintextValue
			}
		case "bearer":
			st.headers["Authorization"] = "Bearer " + secret.PlaintextValue
		}
	}
	return nil, nil
}

func validateSchema(rawSchema []byte, value any) error {
	if len(rawSchema) == 0 {
		return nil
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", bytes.NewReader(rawSchema)); err != nil {
		return err
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return err
	}
	return compiled.Validate(value)
}

func clampTimeoutMS(requested int) int {
	val := requested
	if val <= 0 {
		val = defaultTimeoutMS
	}
	if val < minTimeoutMS {
		val = minTimeoutMS
	}
	if val > maxTimeoutMS {
		val = maxTimeoutMS
	}
	return val
}

func mapRunError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeoutExceeded
	}
	return err
}
