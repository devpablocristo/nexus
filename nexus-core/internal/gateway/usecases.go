package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	auditdomain "nexus-core/internal/audit/usecases/domain"
	"nexus-core/internal/dlp"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/internal/policy"
	policydomain "nexus-core/internal/policy/usecases/domain"
	secretdomain "nexus-core/internal/secrets/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus-core/pkg/types"
	"nexus-core/pkg/utils"
	"nexus-core/pkg/validations/jsonschema"
)

type ToolRepoPort interface {
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
}

type PolicyRepoPort interface {
	ListByToolID(ctx context.Context, orgID, toolID uuid.UUID) ([]policydomain.Policy, error)
}

type AuditRepoPort interface {
	Create(ctx context.Context, ev auditdomain.AuditEvent) error
}

type SecretRepoPort interface {
	ListForTool(ctx context.Context, orgID, toolID uuid.UUID) ([]secretdomain.ToolSecret, error)
}

type EgressPort interface {
	IsHostAllowed(ctx context.Context, orgID, toolID uuid.UUID, host string) (bool, error)
}

type RateLimiterPort interface {
	Allow(key string, perMinute int) bool
}

type HTTPExecutorPort interface {
	Execute(ctx context.Context, method, url string, input map[string]any, headers map[string]string, maxRetries int) (any, int, *types.HTTPError)
}

type IdempotencyPort interface {
	Get(ctx context.Context, orgID uuid.UUID, toolName, key string) (*gwdomain.IdempotencyRecord, error)
	CreateInProgress(ctx context.Context, rec gwdomain.IdempotencyRecord) (bool, error)
	MarkCompleted(ctx context.Context, orgID uuid.UUID, toolName, key string, responseRedacted map[string]any) error
	MarkFailed(ctx context.Context, orgID uuid.UUID, toolName, key string, code *string, responseRedacted map[string]any) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

type RunMetricsPort interface {
	ObserveRun(ctx context.Context, toolName, decision, status string, latency time.Duration)
}

type TenantLimitsPort interface {
	GetRunRPM(ctx context.Context, orgID uuid.UUID) (int, error)
}

type RuntimeActionOverrides struct {
	Deny              bool
	DenyReason        string
	TenantRPMOverride *int
	ToolRPMOverride   *int
}

type ActionOverridesPort interface {
	ResolveRuntimeOverrides(ctx context.Context, orgID uuid.UUID, toolName string) (RuntimeActionOverrides, error)
}

type Service interface {
	Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
	Simulate(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.SimulateResponse, error)
}

type Config struct {
	DefaultRateLimitPerMinute   int
	MaxBytesInputDefault        int
	MaxBytesContextDefault      int
	IdempotencyTTLHours         int
	IdempotencyStalenessSeconds int
	TimeoutBudgetDefaultMS      int
	TimeoutBudgetMinMS          int
	TimeoutBudgetMaxMS          int
	HTTPRetries                 int
	DisableSSRFProtection       bool
	EgressAllowlist             string
	SimEngineBaseURL            string
	SimEngineInternalKey        string
}

// runState agrupa el estado compartido del pipeline Run entre las funciones auxiliares.
type runState struct {
	start                 time.Time
	requestID             string
	budget                *gwdomain.TimeoutBudget
	input                 map[string]any
	contextMap            map[string]any
	tool                  tooldomain.Tool
	isWrite               bool
	idemMeta              gwdomain.IdempotencyMeta
	idempotencyKey        string
	requestFingerprint    string
	createdIdempotencyRow bool
	dlpSummary            map[string]any
	matchReason           string
	policyID              *uuid.UUID
	limits                parsedLimits
	runtimeOverrides      RuntimeActionOverrides
	headers               map[string]string
	remainingBeforeExec   int
	latency               int64
	result                any
	execErr               *types.HTTPError
}

type service struct {
	toolRepo        ToolRepoPort
	policyRepo      PolicyRepoPort
	auditRepo       AuditRepoPort
	secretRepo      SecretRepoPort
	egress          EgressPort
	limiter         RateLimiterPort
	executor        HTTPExecutorPort
	idempotency     IdempotencyPort
	tenantCaps      TenantLimitsPort
	actionOverrides ActionOverridesPort
	metrics         RunMetricsPort
	cache           *jsonschema.CompilerCache
	evaluator       *policy.Evaluator
	dlp             *dlp.Detector
	cfg             Config
	log             zerolog.Logger
}

func NewService(toolRepo ToolRepoPort, policyRepo PolicyRepoPort, auditRepo AuditRepoPort, secretRepo SecretRepoPort, egress EgressPort, limiter RateLimiterPort, executor HTTPExecutorPort, idempotency IdempotencyPort, tenantCaps TenantLimitsPort, actionOverrides ActionOverridesPort, metrics RunMetricsPort, cache *jsonschema.CompilerCache, evaluator *policy.Evaluator, detector *dlp.Detector, cfg Config, log zerolog.Logger) Service {
	if cfg.DisableSSRFProtection {
		log.Warn().Msg("SSRF protection is DISABLED — this must only be used in test/dev environments")
	}
	return &service{
		toolRepo:        toolRepo,
		policyRepo:      policyRepo,
		auditRepo:       auditRepo,
		secretRepo:      secretRepo,
		egress:          egress,
		limiter:         limiter,
		executor:        executor,
		idempotency:     idempotency,
		tenantCaps:      tenantCaps,
		actionOverrides: actionOverrides,
		metrics:         metrics,
		cache:           cache,
		evaluator:       evaluator,
		dlp:             detector,
		cfg:             cfg,
		log:             log,
	}
}

func (s *service) Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	// 1. Inicialización
	start := time.Now()
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.NewString()
	}
	budget := gwdomain.NewTimeoutBudget(gwdomain.ClampTimeoutMS(req.TimeoutMS, s.cfg.TimeoutBudgetDefaultMS, s.cfg.TimeoutBudgetMinMS, s.cfg.TimeoutBudgetMaxMS))
	req.TimeoutMS = budget.TimeoutMS()

	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	contextMap := req.Context
	if contextMap == nil {
		contextMap = map[string]any{}
	}

	// 2. Resolución de tool
	tool, err := s.toolRepo.GetByName(ctx, orgID, req.ToolName)
	if err != nil {
		var he types.HTTPError
		if errors.As(err, &he) && he.Code == types.ErrCodeNotFound {
			// Can't write audit without a valid tool_id due to FK constraints.
			reason := "tool not found"
			code := types.ErrCodeNotFound
			latency := time.Since(start).Milliseconds()
			s.observeRun(ctx, req.ToolName, string(gwdomain.DecisionDeny), string(gwdomain.RunStatusBlocked), time.Duration(latency)*time.Millisecond)
			s.log.Warn().
				Str("request_id", requestID).
				Str("org_id", orgID.String()).
				Str("tool_name", req.ToolName).
				Str("decision", "deny").
				Str("status", "blocked").
				Int64("latency_ms", latency).
				Str("error_code", code).
				Msg("run_blocked")
			return gwdomain.RunResponse{
				RequestID:  requestID,
				Decision:   gwdomain.DecisionDeny,
				ToolName:   req.ToolName,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     &reason,
				ErrorCode:  &code,
				ErrorMsg:   &reason,
				LatencyMS:  latency,
				HTTPStatus: http.StatusNotFound,
			}, nil
		}
		s.log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("org_id", orgID.String()).
			Str("tool_name", req.ToolName).
			Msg("tool_lookup_failed")
		return gwdomain.RunResponse{}, err
	}

	// 3. Idempotencia
	isWrite := tool.ActionType == tooldomain.ActionWrite || (tool.ActionType == "" && strings.ToUpper(tool.Method) != "GET")
	idemMeta := gwdomain.IdempotencyMeta{Present: req.IdempotencyKey != nil, Outcome: gwdomain.IdempotencySkippedNotWrite}
	st := &runState{
		start: start, requestID: requestID, budget: budget, input: input, contextMap: contextMap,
		tool: tool, isWrite: isWrite, idemMeta: idemMeta,
	}
	if resp, err := s.runResolveIdempotency(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}

	// 4-7. Validación tool, contexto, DLP, schema entrada
	if resp, err := s.runValidateAndPrepare(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}
	// 8-9. Políticas y decisión allow/deny
	if resp, err := s.runPoliciesAndDecision(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}
	// 10-14. Action overrides, rate limits, URL/egress, secretos
	if resp, err := s.runOverridesRateLimitsEgressSecrets(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}
	// 15-19. Timeout, ejecución HTTP, schema salida, auditoría, respuesta
	return s.runExecuteAndFinish(ctx, orgID, req, st)
}

// runResolveIdempotency implementa:
//  3. Idempotencia
func (s *service) runResolveIdempotency(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	if !st.isWrite || req.IdempotencyKey == nil {
		return nil, nil
	}
	st.idempotencyKey = *req.IdempotencyKey
	st.idemMeta.Outcome = gwdomain.IdempotencyNew
	var err error
	st.requestFingerprint, err = buildRequestFingerprint(req.ToolName, st.input, req.Actor, req.Role, req.Scopes)
	if err != nil {
		return nil, err
	}
	existing, err := s.idempotency.Get(ctx, orgID, st.tool.Name, st.idempotencyKey)
	if err != nil {
		return nil, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store read failed")
	}
	if existing != nil {
		if existing.RequestFingerprint != st.requestFingerprint {
			reason := "idempotency key used with different payload"
			code := types.ErrCodeIdempotencyConflict
			st.idemMeta.Outcome = gwdomain.IdempotencyConflict
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, reason, code, http.StatusConflict, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, nil, nil)
			return &resp, nil
		}
		switch existing.Status {
		case gwdomain.IdempotencyStatusCompleted:
			st.idemMeta.Outcome = gwdomain.IdempotencyReplay
			latency := time.Since(st.start).Milliseconds()
			var result any
			var status gwdomain.RunStatus = gwdomain.RunStatusSuccess
			var decision gwdomain.Decision = gwdomain.DecisionAllow
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
			_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
				OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
				Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
				InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
				DLPSummary: map[string]any{}, Decision: auditdomain.Decision(decision), Reason: reason,
				Status: auditdomain.Status(status), OutputRedacted: utils.Redact(result),
				ErrorCode: errCode, ErrorMessage: errMsg, LatencyMS: int(latency),
				IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
				TimeoutMS: intPtr(req.TimeoutMS),
			})
			s.observeRun(ctx, st.tool.Name, string(decision), string(status), time.Duration(latency)*time.Millisecond)
			httpStatus := http.StatusOK
			if status == gwdomain.RunStatusError {
				httpStatus = http.StatusBadGateway
			}
			if status == gwdomain.RunStatusBlocked {
				httpStatus = http.StatusForbidden
			}
			resp := gwdomain.RunResponse{
				RequestID: st.requestID, Decision: decision, ToolName: st.tool.Name, Status: status,
				Reason: reason, Result: result, ErrorCode: errCode, ErrorMsg: errMsg,
				LatencyMS: latency, HTTPStatus: httpStatus, Idempotency: st.idemMeta,
			}
			return &resp, nil
		case gwdomain.IdempotencyStatusInProgress:
			staleness := time.Duration(max(1, s.cfg.IdempotencyStalenessSeconds)) * time.Second
			if !existing.CreatedAt.IsZero() && time.Since(existing.CreatedAt) > staleness {
				_ = s.idempotency.MarkFailed(ctx, orgID, st.tool.Name, st.idempotencyKey, ptr(types.ErrCodeTimeout), map[string]any{
					"status": "error", "decision": "allow", "reason": "stale in-progress record expired",
				})
				st.idemMeta.Outcome = gwdomain.IdempotencyNew
				inserted, createErr := s.idempotency.CreateInProgress(ctx, gwdomain.IdempotencyRecord{
					OrgID: orgID, ToolName: st.tool.Name, IdempotencyKey: st.idempotencyKey,
					RequestFingerprint: st.requestFingerprint, Status: gwdomain.IdempotencyStatusInProgress,
					ExpiresAt: time.Now().UTC().Add(time.Duration(max(1, s.cfg.IdempotencyTTLHours)) * time.Hour),
				})
				if createErr != nil {
					return nil, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store write failed")
				}
				if !inserted {
					return nil, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store write failed after stale cleanup")
				}
				st.createdIdempotencyRow = true
			} else {
				st.idemMeta.Outcome = gwdomain.IdempotencyInProgress
				reason := "idempotency request in progress"
				code := types.ErrCodeIdempotencyProgress
				resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, reason, code, http.StatusConflict, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, nil, nil)
				return &resp, nil
			}
		case gwdomain.IdempotencyStatusFailed:
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
			_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
				OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
				Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
				InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
				DLPSummary: map[string]any{}, Decision: auditdomain.Decision(decision), Reason: reason,
				Status: auditdomain.Status(status), ErrorCode: &errCode, ErrorMessage: &errMsg,
				LatencyMS: int(latency), IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
				TimeoutMS: intPtr(req.TimeoutMS),
			})
			s.observeRun(ctx, st.tool.Name, string(decision), string(status), time.Duration(latency)*time.Millisecond)
			resp := gwdomain.RunResponse{
				RequestID: st.requestID, Decision: decision, ToolName: st.tool.Name, Status: status,
				Reason: reason, ErrorCode: &errCode, ErrorMsg: &errMsg, LatencyMS: latency,
				HTTPStatus: httpStatus, Idempotency: st.idemMeta,
			}
			return &resp, nil
		}
	} else {
		inserted, err := s.idempotency.CreateInProgress(ctx, gwdomain.IdempotencyRecord{
			OrgID: orgID, ToolName: st.tool.Name, IdempotencyKey: st.idempotencyKey,
			RequestFingerprint: st.requestFingerprint, Status: gwdomain.IdempotencyStatusInProgress,
			ExpiresAt: time.Now().UTC().Add(time.Duration(max(1, s.cfg.IdempotencyTTLHours)) * time.Hour),
		})
		if err != nil {
			return nil, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store write failed")
		}
		if !inserted {
			st.idemMeta.Outcome = gwdomain.IdempotencyInProgress
			reason := "idempotency request in progress"
			code := types.ErrCodeIdempotencyProgress
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, reason, code, http.StatusConflict, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, nil, nil)
			return &resp, nil
		}
		st.createdIdempotencyRow = true
	}
	return nil, nil
}

// runValidateAndPrepare implementa:
//  4. Validación tool (enabled, kind)
//  5. Contexto para políticas (actor, role, scopes, auth_method)
//  6. DLP
//  7. Validación schema de entrada
func (s *service) runValidateAndPrepare(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	// 4. Validación tool (enabled, kind)
	if !st.tool.Enabled {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "tool disabled", types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	if st.tool.Kind != tooldomain.ToolKindHTTP {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "unsupported tool kind", types.ErrCodeValidation, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	// 5. Contexto para políticas (actor, role, scopes, auth_method)
	if req.Actor != nil && *req.Actor != "" {
		st.contextMap["actor"] = *req.Actor
	}
	if req.Role != nil && *req.Role != "" {
		st.contextMap["role"] = *req.Role
	}
	if len(req.Scopes) > 0 {
		arr := make([]any, 0, len(req.Scopes))
		for _, scope := range req.Scopes {
			arr = append(arr, scope)
		}
		st.contextMap["scopes"] = arr
	}
	if req.AuthMethod != "" {
		st.contextMap["auth_method"] = req.AuthMethod
	}
	// 6. DLP
	st.dlpSummary = s.dlp.Summarize(st.input, st.contextMap)
	st.contextMap["dlp"] = st.dlpSummary
	st.budget.Consume("dlp", time.Since(st.start))

	// 7. Validación schema de entrada
	schemaStart := time.Now()
	inSchema, err := s.cache.Compile(ctx, st.tool.ID.String()+":in", st.tool.InputSchemaJSON)
	if err != nil {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "tool input schema invalid", types.ErrCodeSchemaInvalid, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	if err := jsonschema.Validate(inSchema, st.input); err != nil {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "input does not match schema", types.ErrCodeValidation, http.StatusBadRequest, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	st.budget.Consume("schema_validation", time.Since(schemaStart))
	return nil, nil
}

// runPoliciesAndDecision implementa:
//  8. Políticas y decisión allow/deny (firstMatch, límites input/context)
//  9. Aplicar decisión deny
func (s *service) runPoliciesAndDecision(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	// 8. Políticas y decisión allow/deny (firstMatch, límites input/context)
	policies, err := s.policyRepo.ListByToolID(ctx, orgID, st.tool.ID)
	if err != nil {
		return nil, err
	}
	match, matchReason, limits, err := s.firstMatch(policies, st.input, st.contextMap, st.tool)
	if err != nil {
		return nil, err
	}
	st.matchReason = matchReason
	st.limits = limits
	if st.isWrite && req.IdempotencyKey == nil && limits.requireIdempotency() {
		st.idemMeta.Outcome = gwdomain.IdempotencyMissingRequired
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "idempotency key required by policy", types.ErrCodeIdempotencyRequired, http.StatusBadRequest, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		return &resp, nil
	}
	decision := gwdomain.DecisionAllow
	var policyID *uuid.UUID
	if match != nil {
		id := match.ID
		policyID = &id
		st.policyID = &id
		if match.Effect == policydomain.EffectDeny {
			decision = gwdomain.DecisionDeny
		}
	} else {
		st.policyID = nil
		if st.tool.ActionType == tooldomain.ActionWrite {
			decision = gwdomain.DecisionDeny
			st.matchReason = "default deny for write tool"
		} else {
			st.matchReason = "default allow for read tool"
		}
	}
	if decision == gwdomain.DecisionAllow {
		if maxIn := limits.maxBytesInput(s.cfg.MaxBytesInputDefault); maxIn > 0 {
			n, err := utils.JSONSize(st.input)
			if err != nil {
				return nil, err
			}
			if n > maxIn {
				resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, policyID, "input too large", types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
				s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
				return &resp, nil
			}
		}
		if maxCtx := limits.maxBytesContext(s.cfg.MaxBytesContextDefault); maxCtx > 0 {
			n, err := utils.JSONSize(st.contextMap)
			if err != nil {
				return nil, err
			}
			if n > maxCtx {
				resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, policyID, "context too large", types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
				s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
				return &resp, nil
			}
		}
	}
	// 9. Aplicar decisión deny
	if decision == gwdomain.DecisionDeny {
		reason := st.matchReason
		code := types.ErrCodePolicyDenied
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, reason, code, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	return nil, nil
}

// runOverridesRateLimitsEgressSecrets implementa:
// 10. Action overrides
// 11. Rate limit tenant
// 12. Rate limit por tool
// 13. URL y egress (SSRF, allowlist)
// 14. Secretos (headers/bearer)
func (s *service) runOverridesRateLimitsEgressSecrets(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	// 10. Action overrides
	var err error
	st.runtimeOverrides = RuntimeActionOverrides{}
	if s.actionOverrides != nil {
		st.runtimeOverrides, err = s.actionOverrides.ResolveRuntimeOverrides(ctx, orgID, st.tool.Name)
		if err != nil {
			return nil, err
		}
		if st.runtimeOverrides.Deny {
			reason := st.runtimeOverrides.DenyReason
			if strings.TrimSpace(reason) == "" {
				reason = "blocked by active action override"
			}
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, reason, types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
			s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
			return &resp, nil
		}
	}
	// 11. Rate limit tenant
	tenantRunRPM := 0
	if s.tenantCaps != nil {
		tenantRunRPM, err = s.tenantCaps.GetRunRPM(ctx, orgID)
		if err != nil {
			return nil, err
		}
	}
	if st.runtimeOverrides.TenantRPMOverride != nil && (*st.runtimeOverrides.TenantRPMOverride > 0) {
		if tenantRunRPM <= 0 || *st.runtimeOverrides.TenantRPMOverride < tenantRunRPM {
			tenantRunRPM = *st.runtimeOverrides.TenantRPMOverride
		}
	}
	if tenantRunRPM > 0 {
		tenantKey := orgID.String() + ":tenant"
		if !s.limiter.Allow(tenantKey, tenantRunRPM) {
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, "tenant run rate limit exceeded (bucket=org:tenant limit_per_minute="+strconv.Itoa(tenantRunRPM)+")", types.ErrCodeRateLimited, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
			s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
			return &resp, nil
		}
	}
	// 12. Rate limit por tool
	perMin := st.limits.rateLimitPerMinute(s.cfg.DefaultRateLimitPerMinute)
	if st.runtimeOverrides.ToolRPMOverride != nil && (*st.runtimeOverrides.ToolRPMOverride > 0) {
		if perMin <= 0 || *st.runtimeOverrides.ToolRPMOverride < perMin {
			perMin = *st.runtimeOverrides.ToolRPMOverride
		}
	}
	if perMin > 0 {
		key := orgID.String() + ":" + st.tool.Name
		if !s.limiter.Allow(key, perMin) {
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, "rate limit exceeded (bucket=org+tool limit_per_minute="+strconv.Itoa(perMin)+")", types.ErrCodeRateLimited, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
			s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
			return &resp, nil
		}
	}
	// 13. URL y egress (SSRF, allowlist)
	u, parseErr := url.Parse(st.tool.URL)
	if parseErr != nil || u.Hostname() == "" {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, "invalid tool url", types.ErrCodeValidation, http.StatusBadRequest, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	if !s.cfg.DisableSSRFProtection {
		if err := utils.ValidateEgressURLWithAllowlist(st.tool.URL, s.cfg.EgressAllowlist); err != nil {
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, "ssrf blocked: "+err.Error(), types.ErrCodeEgressDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
			s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
			return &resp, nil
		}
	}
	allowed, err := s.egress.IsHostAllowed(ctx, orgID, st.tool.ID, strings.ToLower(u.Hostname()))
	if err != nil {
		return nil, err
	}
	if !allowed {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, "egress host denied", types.ErrCodeEgressDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	// 14. Secretos (headers/bearer)
	st.headers = map[string]string{}
	secrets, err := s.secretRepo.ListForTool(ctx, orgID, st.tool.ID)
	if err != nil {
		return nil, err
	}
	for _, secret := range secrets {
		if !secret.Enabled {
			continue
		}
		if strings.EqualFold(secret.SecretType, "header") && secret.KeyName != "" {
			st.headers[secret.KeyName] = secret.PlaintextValue
		}
		if strings.EqualFold(secret.SecretType, "bearer") {
			st.headers["Authorization"] = "Bearer " + secret.PlaintextValue
		}
	}
	// Propagate canonical request_id to upstream providers for end-to-end traceability.
	st.headers["X-Nexus-Request-Id"] = st.requestID
	// Send Core->SimEngine shared secret only when target is the configured SimEngine provider.
	if strings.TrimSpace(s.cfg.SimEngineInternalKey) != "" && s.isSimEngineToolURL(st.tool.URL) {
		st.headers["X-Sim-Engine-Internal-Key"] = s.cfg.SimEngineInternalKey
	}
	return nil, nil
}

// runExecuteAndFinish implementa:
// 15. Timeout budget antes de ejecutar
// 16. Ejecución HTTP al upstream
// 17. Validación schema de salida (best-effort)
// 18. Manejo error de ejecución
// 19. Éxito: auditoría, idempotency completed, respuesta
func (s *service) runExecuteAndFinish(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (gwdomain.RunResponse, error) {
	// 15. Timeout budget antes de ejecutar
	st.remainingBeforeExec = st.budget.RemainingMS()
	if st.remainingBeforeExec <= 0 {
		code := types.ErrCodeTimeoutBudget
		reason := "timeout budget exhausted before execute"
		resp := s.errorRun(ctx, orgID, req, st.tool, st.requestID, st.policyID, st.matchReason, reason, &code, &reason, http.StatusRequestTimeout, st.start, st.input, st.contextMap, st.dlpSummary, st.idemMeta, req.TimeoutMS, intPtr(0), st.budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return resp, nil
	}
	execTimeoutMS := st.remainingBeforeExec
	execCtx, cancelExec := context.WithTimeout(ctx, time.Duration(execTimeoutMS)*time.Millisecond)
	execStart := time.Now()
	maxRetries := s.cfg.HTTPRetries
	if st.isWrite {
		maxRetries = 0
	}
	// 16. Ejecución HTTP al upstream
	st.result, _, st.execErr = s.executor.Execute(execCtx, st.tool.Method, st.tool.URL, st.input, st.headers, maxRetries)
	cancelExec()
	st.budget.Consume("execute_http", time.Since(execStart))
	st.latency = time.Since(st.start).Milliseconds()

	// 17. Validación schema de salida (best-effort)
	if st.execErr == nil && len(st.tool.OutputSchemaJSON) > 0 {
		outSchema, err := s.cache.Compile(ctx, st.tool.ID.String()+":out", st.tool.OutputSchemaJSON)
		if err != nil || jsonschema.Validate(outSchema, st.result) != nil {
			code := types.ErrCodeOutputSchemaInvalid
			msg := "tool output does not match schema"
			_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
				OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
				Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
				InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
				DLPSummary: st.dlpSummary, Decision: auditdomain.DecisionAllow, PolicyID: st.policyID,
				Reason: strPtr(st.matchReason), Status: auditdomain.StatusError, OutputRedacted: utils.Redact(st.result),
				ErrorCode: &code, ErrorMessage: &msg, LatencyMS: int(st.latency),
				IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
				TimeoutMS: intPtr(req.TimeoutMS), BudgetRemainingMSAtExecute: intPtr(st.remainingBeforeExec),
				StageDurationsMS: st.budget.StageDurationsMS(),
			})
			resp := gwdomain.RunResponse{Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, HTTPStatus: http.StatusBadGateway}
			s.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
			s.observeRun(ctx, st.tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusError), time.Duration(st.latency)*time.Millisecond)
			return gwdomain.RunResponse{
				RequestID: st.requestID, Decision: gwdomain.DecisionAllow, ToolName: st.tool.Name,
				Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, LatencyMS: st.latency,
				HTTPStatus: http.StatusBadGateway, Idempotency: st.idemMeta,
			}, nil
		}
	}
	// 18. Manejo error de ejecución
	if st.execErr != nil {
		code := st.execErr.Code
		if st.execErr.Code == types.ErrCodeTimeout && st.budget.RemainingMS() <= 0 {
			code = types.ErrCodeTimeoutBudget
		}
		msg := st.execErr.Message
		s.log.Error().
			Str("request_id", st.requestID).Str("org_id", orgID.String()).Str("tool_name", st.tool.Name).
			Str("decision", "allow").Str("status", "error").Str("error_code", code).
			Msg("run_error")
		_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
			OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
			Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
			InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
			DLPSummary: st.dlpSummary, Decision: auditdomain.DecisionAllow, PolicyID: st.policyID,
			Reason: strPtr(st.matchReason), Status: auditdomain.StatusError, OutputRedacted: nil,
			ErrorCode: &code, ErrorMessage: &msg, LatencyMS: int(st.latency),
			IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
			TimeoutMS: intPtr(req.TimeoutMS), BudgetRemainingMSAtExecute: intPtr(st.remainingBeforeExec),
			StageDurationsMS: st.budget.StageDurationsMS(),
		})
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
		}, nil
	}
	// 19. Éxito: auditoría, idempotency completed, respuesta
	_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
		OrgID: orgID, ToolID: st.tool.ID, ToolName: st.tool.Name, RequestID: st.requestID,
		Actor: req.Actor, ActorRole: req.Role, ActorScopes: req.Scopes,
		InputRedacted: utils.Redact(st.input), ContextRedacted: utils.Redact(st.contextMap),
		DLPSummary: st.dlpSummary, Decision: auditdomain.DecisionAllow, PolicyID: st.policyID,
		Reason: strPtr(st.matchReason), Status: auditdomain.StatusSuccess, OutputRedacted: utils.Redact(st.result),
		LatencyMS: int(st.latency), IdempotencyPresent: st.idemMeta.Present, IdempotencyOutcome: string(st.idemMeta.Outcome),
		TimeoutMS: intPtr(req.TimeoutMS), BudgetRemainingMSAtExecute: intPtr(st.remainingBeforeExec),
		StageDurationsMS: st.budget.StageDurationsMS(),
	})
	if st.createdIdempotencyRow {
		_ = s.idempotency.MarkCompleted(ctx, orgID, st.tool.Name, st.idempotencyKey, map[string]any{
			"decision": string(gwdomain.DecisionAllow), "status": string(gwdomain.RunStatusSuccess), "result": utils.Redact(st.result),
		})
	}
	s.log.Info().
		Str("request_id", st.requestID).Str("org_id", orgID.String()).Str("tool_name", st.tool.Name).
		Str("decision", "allow").Str("status", "success").Int64("latency_ms", st.latency).
		Msg("run_success")
	annotateRunSpan(ctx, orgID, st.input, st.tool.Name, st.requestID, "allow", st.policyID)
	s.observeRun(ctx, st.tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusSuccess), time.Duration(st.latency)*time.Millisecond)
	return gwdomain.RunResponse{
		RequestID: st.requestID, Decision: gwdomain.DecisionAllow, ToolName: st.tool.Name,
		Status: gwdomain.RunStatusSuccess, Result: st.result, LatencyMS: st.latency,
		HTTPStatus: http.StatusOK, Idempotency: st.idemMeta,
	}, nil
}

func (s *service) Simulate(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.SimulateResponse, error) {
	start := time.Now()
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.NewString()
	}
	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	contextMap := req.Context
	if contextMap == nil {
		contextMap = map[string]any{}
	}
	explain := map[string]any{"mode": "simulate"}

	tool, err := s.toolRepo.GetByName(ctx, orgID, req.ToolName)
	if err != nil {
		var he types.HTTPError
		if errors.As(err, &he) && he.Code == types.ErrCodeNotFound {
			reason := "tool not found"
			code := types.ErrCodeNotFound
			return gwdomain.SimulateResponse{
				RequestID:  requestID,
				Decision:   gwdomain.DecisionDeny,
				ToolName:   req.ToolName,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     &reason,
				ErrorCode:  &code,
				ErrorMsg:   &reason,
				LatencyMS:  time.Since(start).Milliseconds(),
				HTTPStatus: http.StatusNotFound,
				Explain:    map[string]any{"mode": "simulate", "stage": "tool_lookup", "result": "not_found"},
			}, nil
		}
		return gwdomain.SimulateResponse{}, err
	}
	explain["tool_id"] = tool.ID.String()
	explain["tool_name"] = tool.Name

	if !tool.Enabled {
		reason := "tool disabled"
		code := types.ErrCodePolicyDenied
		explain["stage"] = "tool_enabled"
		return gwdomain.SimulateResponse{
			RequestID:  requestID,
			Decision:   gwdomain.DecisionDeny,
			ToolName:   tool.Name,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			LatencyMS:  time.Since(start).Milliseconds(),
			HTTPStatus: http.StatusForbidden,
			Explain:    explain,
		}, nil
	}

	if req.Actor != nil && *req.Actor != "" {
		contextMap["actor"] = *req.Actor
	}
	if req.Role != nil && *req.Role != "" {
		contextMap["role"] = *req.Role
	}
	if len(req.Scopes) > 0 {
		arr := make([]any, 0, len(req.Scopes))
		for _, scope := range req.Scopes {
			arr = append(arr, scope)
		}
		contextMap["scopes"] = arr
	}
	dlpSummary := s.dlp.Summarize(input, contextMap)
	contextMap["dlp"] = dlpSummary
	explain["dlp_summary"] = dlpSummary

	inSchema, err := s.cache.Compile(ctx, tool.ID.String()+":in", tool.InputSchemaJSON)
	if err != nil {
		reason := "tool input schema invalid"
		code := types.ErrCodeSchemaInvalid
		explain["stage"] = "schema_compile"
		return gwdomain.SimulateResponse{
			RequestID:  requestID,
			Decision:   gwdomain.DecisionDeny,
			ToolName:   tool.Name,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			LatencyMS:  time.Since(start).Milliseconds(),
			HTTPStatus: http.StatusForbidden,
			Explain:    explain,
		}, nil
	}
	if err := jsonschema.Validate(inSchema, input); err != nil {
		reason := "input does not match schema"
		code := types.ErrCodeValidation
		explain["stage"] = "schema_validate"
		return gwdomain.SimulateResponse{
			RequestID:  requestID,
			Decision:   gwdomain.DecisionDeny,
			ToolName:   tool.Name,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			LatencyMS:  time.Since(start).Milliseconds(),
			HTTPStatus: http.StatusBadRequest,
			Explain:    explain,
		}, nil
	}

	policies, err := s.policyRepo.ListByToolID(ctx, orgID, tool.ID)
	if err != nil {
		return gwdomain.SimulateResponse{}, err
	}
	match, matchReason, limits, err := s.firstMatch(policies, input, contextMap, tool)
	if err != nil {
		return gwdomain.SimulateResponse{}, err
	}
	decision := gwdomain.DecisionAllow
	var policyID *uuid.UUID
	if match != nil {
		id := match.ID
		policyID = &id
		explain["matched_policy_id"] = id.String()
		explain["matched_policy_effect"] = string(match.Effect)
		explain["matched_policy_priority"] = match.Priority
		if match.Effect == policydomain.EffectDeny {
			decision = gwdomain.DecisionDeny
		}
	} else if tool.ActionType == tooldomain.ActionWrite {
		decision = gwdomain.DecisionDeny
		matchReason = "default deny for write tool"
		explain["default_decision"] = "deny"
	} else {
		matchReason = "default allow for read tool"
		explain["default_decision"] = "allow"
	}

	if decision == gwdomain.DecisionAllow {
		if maxIn := limits.maxBytesInput(s.cfg.MaxBytesInputDefault); maxIn > 0 {
			n, _ := utils.JSONSize(input)
			explain["input_bytes"] = n
			explain["max_bytes_input"] = maxIn
			if n > maxIn {
				decision = gwdomain.DecisionDeny
				matchReason = "input too large"
			}
		}
		if maxCtx := limits.maxBytesContext(s.cfg.MaxBytesContextDefault); maxCtx > 0 {
			n, _ := utils.JSONSize(contextMap)
			explain["context_bytes"] = n
			explain["max_bytes_context"] = maxCtx
			if n > maxCtx {
				decision = gwdomain.DecisionDeny
				matchReason = "context too large"
			}
		}
	}

	u, parseErr := url.Parse(tool.URL)
	if parseErr == nil && u.Hostname() != "" {
		host := strings.ToLower(u.Hostname())
		explain["egress_host"] = host
		allowed, err := s.egress.IsHostAllowed(ctx, orgID, tool.ID, host)
		if err != nil {
			return gwdomain.SimulateResponse{}, err
		}
		explain["egress_allowed"] = allowed
		if !allowed {
			decision = gwdomain.DecisionDeny
			matchReason = "egress host denied"
		}
	}
	secrets, err := s.secretRepo.ListForTool(ctx, orgID, tool.ID)
	if err != nil {
		return gwdomain.SimulateResponse{}, err
	}
	explain["secret_count"] = len(secrets)
	explain["rate_limit_checked"] = false
	explain["would_execute"] = decision == gwdomain.DecisionAllow
	explain["policy_id"] = ""
	if policyID != nil {
		explain["policy_id"] = policyID.String()
	}

	latency := time.Since(start).Milliseconds()
	if decision == gwdomain.DecisionDeny {
		code := types.ErrCodePolicyDenied
		if matchReason == "egress host denied" {
			code = types.ErrCodeEgressDenied
		}
		return gwdomain.SimulateResponse{
			RequestID:  requestID,
			Decision:   gwdomain.DecisionDeny,
			ToolName:   tool.Name,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &matchReason,
			ErrorCode:  &code,
			ErrorMsg:   &matchReason,
			LatencyMS:  latency,
			HTTPStatus: http.StatusForbidden,
			Explain:    explain,
		}, nil
	}

	return gwdomain.SimulateResponse{
		RequestID:  requestID,
		Decision:   gwdomain.DecisionAllow,
		ToolName:   tool.Name,
		Status:     gwdomain.RunStatusSuccess,
		Reason:     strPtr(matchReason),
		LatencyMS:  latency,
		HTTPStatus: http.StatusOK,
		Explain:    explain,
	}, nil
}

type parsedLimits struct {
	m map[string]any
}

func (l parsedLimits) rateLimitPerMinute(def int) int {
	if l.m == nil {
		return def
	}
	rl, ok := l.m["rate_limit"].(map[string]any)
	if !ok {
		return def
	}
	v, ok := rl["per_minute"]
	if !ok {
		return def
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return def
}

func (l parsedLimits) maxBytesInput(def int) int {
	if l.m == nil {
		return def
	}
	if v, ok := l.m["max_bytes_input"].(float64); ok {
		return int(v)
	}
	return def
}

func (l parsedLimits) maxBytesContext(def int) int {
	if l.m == nil {
		return def
	}
	if v, ok := l.m["max_bytes_context"].(float64); ok {
		return int(v)
	}
	return def
}

func (l parsedLimits) requireIdempotency() bool {
	if l.m == nil {
		return false
	}
	v, ok := l.m["require_idempotency"]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	default:
		return false
	}
}

func (s *service) firstMatch(policies []policydomain.Policy, input, contextMap map[string]any, tool tooldomain.Tool) (*policydomain.Policy, string, parsedLimits, error) {
	attrs := policy.ToolAttributes{
		Name:           tool.Name,
		Kind:           string(tool.Kind),
		Method:         tool.Method,
		URL:            tool.URL,
		ActionType:     string(tool.ActionType),
		Classification: tool.Classification,
		Sensitivity:    tool.Sensitivity,
		RiskLevel:      tool.RiskLevel,
	}
	for _, p := range policies {
		ok, err := s.evaluator.Matches(p.ConditionsJSON, input, contextMap, attrs)
		if err != nil {
			return nil, "", parsedLimits{}, err
		}
		if !ok {
			continue
		}
		var lim map[string]any
		_ = json.Unmarshal(p.LimitsJSON, &lim)
		reason := p.ReasonTemplate
		if reason == "" {
			reason = "matched policy"
		}
		return &p, reason, parsedLimits{m: lim}, nil
	}
	return nil, "", parsedLimits{}, nil
}

func (s *service) blocked(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, requestID string, toolName string, toolIDVal uuid.UUID, policyID *uuid.UUID, reason string, code string, httpStatus int, start time.Time, input, contextMap map[string]any, idem gwdomain.IdempotencyMeta, timeoutMS int, budgetRemaining *int, stageDur map[string]int64) gwdomain.RunResponse {
	latency := time.Since(start).Milliseconds()
	decision := auditdomain.DecisionDeny
	status := auditdomain.StatusBlocked
	rc := reason
	_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
		OrgID:                      orgID,
		ToolID:                     toolIDVal,
		ToolName:                   toolName,
		RequestID:                  requestID,
		Actor:                      actor,
		ActorRole:                  role,
		ActorScopes:                scopes,
		InputRedacted:              utils.Redact(input),
		ContextRedacted:            utils.Redact(contextMap),
		DLPSummary:                 contextMap["dlp"],
		Decision:                   decision,
		PolicyID:                   policyID,
		Reason:                     &rc,
		Status:                     status,
		OutputRedacted:             nil,
		ErrorCode:                  &code,
		ErrorMessage:               &rc,
		LatencyMS:                  int(latency),
		IdempotencyPresent:         idem.Present,
		IdempotencyOutcome:         string(idem.Outcome),
		TimeoutMS:                  intPtr(timeoutMS),
		BudgetRemainingMSAtExecute: budgetRemaining,
		StageDurationsMS:           stageDur,
	})
	s.log.Warn().
		Str("request_id", requestID).
		Str("org_id", orgID.String()).
		Str("tool_name", toolName).
		Str("decision", "deny").
		Str("status", "blocked").
		Int64("latency_ms", latency).
		Str("error_code", code).
		Msg("run_blocked")
	s.emitWorldEnforcementEvent(ctx, orgID, requestID, toolName, code, policyID, reason, input)
	annotateRunSpan(ctx, orgID, input, toolName, requestID, "deny", policyID)
	s.observeRun(ctx, toolName, string(gwdomain.DecisionDeny), string(gwdomain.RunStatusBlocked), time.Duration(latency)*time.Millisecond)
	return gwdomain.RunResponse{
		RequestID:   requestID,
		Decision:    gwdomain.DecisionDeny,
		ToolName:    toolName,
		Status:      gwdomain.RunStatusBlocked,
		Reason:      &reason,
		ErrorCode:   &code,
		ErrorMsg:    &reason,
		LatencyMS:   latency,
		HTTPStatus:  httpStatus,
		Idempotency: idem,
	}
}

func (s *service) emitWorldEnforcementEvent(ctx context.Context, orgID uuid.UUID, requestID, toolName, code string, policyID *uuid.UUID, reason string, input map[string]any) {
	if !strings.HasPrefix(toolName, "world.") {
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.SimEngineBaseURL), "/")
	if baseURL == "" {
		return
	}
	runID := strings.TrimSpace(asString(input["run_id"]))
	if runID == "" {
		return
	}
	eventType := ""
	switch code {
	case types.ErrCodePolicyDenied:
		eventType = "tool.denied"
	case types.ErrCodeRateLimited:
		eventType = "tool.rate_limited"
	default:
		return
	}
	agentID := strings.TrimSpace(asString(input["agent_id"]))
	orgIDStr := strings.TrimSpace(asString(input["org_id"]))
	if orgIDStr == "" {
		orgIDStr = orgID.String()
	}
	stepID := toInt64Any(input["step_id"])
	payload := map[string]any{
		"org_id":     orgIDStr,
		"run_id":     runID,
		"step_id":    stepID,
		"agent_id":   agentID,
		"tool_name":  toolName,
		"request_id": requestID,
		"event_type": eventType,
		"detail":     reason,
	}
	if eventType == "tool.rate_limited" {
		payload["bucket"] = "org+tool"
	}
	if policyID != nil {
		payload["policy_id"] = policyID.String()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	upstreamCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, baseURL+"/admin/run/enforcement", bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set("X-Nexus-Request-Id", requestID)
	}
	if strings.TrimSpace(s.cfg.SimEngineInternalKey) != "" {
		req.Header.Set("X-Sim-Engine-Internal-Key", s.cfg.SimEngineInternalKey)
	}
	resp, err := (&http.Client{Timeout: 1500 * time.Millisecond}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func toInt64Any(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return n
		}
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
			return n
		}
	}
	return 0
}

func annotateRunSpan(ctx context.Context, orgID uuid.UUID, input map[string]any, toolName, requestID, decision string, policyID *uuid.UUID) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	agentID := strings.TrimSpace(asString(input["agent_id"]))
	runID := strings.TrimSpace(asString(input["run_id"]))
	stepID := toInt64Any(input["step_id"])
	attrs := []attribute.KeyValue{
		attribute.String("nexus.org_id", orgID.String()),
		attribute.String("nexus.agent_id", agentID),
		attribute.String("nexus.run_id", runID),
		attribute.Int64("nexus.step_id", stepID),
		attribute.String("nexus.tool_name", toolName),
		attribute.String("nexus.request_id", requestID),
		attribute.String("nexus.decision", decision),
	}
	if policyID != nil {
		attrs = append(attrs, attribute.String("nexus.policy_id", policyID.String()))
	}
	span.SetAttributes(attrs...)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptr(s string) *string {
	return &s
}

func (s *service) observeRun(ctx context.Context, toolName, decision, status string, latency time.Duration) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveRun(ctx, toolName, decision, status, latency)
}

func (s *service) failIdempotencyIfNeeded(ctx context.Context, shouldUpdate bool, orgID uuid.UUID, toolName, key string, resp *gwdomain.RunResponse) {
	if !shouldUpdate || key == "" {
		return
	}
	var code *string
	response := map[string]any{}
	if resp != nil {
		response["decision"] = string(resp.Decision)
		response["status"] = string(resp.Status)
		response["http_status"] = resp.HTTPStatus
		if resp.Result != nil {
			response["result"] = utils.Redact(resp.Result)
		}
		if resp.Reason != nil && *resp.Reason != "" {
			response["reason"] = *resp.Reason
		}
		if resp.ErrorCode != nil || resp.ErrorMsg != nil {
			errorObj := map[string]any{}
			if resp.ErrorCode != nil && *resp.ErrorCode != "" {
				errorObj["code"] = *resp.ErrorCode
				code = resp.ErrorCode
			}
			if resp.ErrorMsg != nil && *resp.ErrorMsg != "" {
				errorObj["message"] = *resp.ErrorMsg
			}
			if len(errorObj) > 0 {
				response["error"] = errorObj
			}
		}
	}
	_ = s.idempotency.MarkFailed(ctx, orgID, toolName, key, code, response)
}

func buildRequestFingerprint(toolName string, input map[string]any, actor, role *string, scopes []string) (string, error) {
	inCanon, err := utils.CanonicalJSON(input)
	if err != nil {
		return "", err
	}
	sortedScopes := append([]string{}, scopes...)
	sort.Strings(sortedScopes)
	scopesCanon, err := utils.CanonicalJSON(sortedScopes)
	if err != nil {
		return "", err
	}
	return utils.FingerprintSHA256(toolName, string(inCanon), ptrString(actor), ptrString(role), string(scopesCanon)), nil
}

func ptrString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func intPtr(v int) *int {
	return &v
}

func (s *service) errorRun(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, tool tooldomain.Tool, requestID string, policyID *uuid.UUID, reasonTemplate, reason string, code, msg *string, httpStatus int, start time.Time, input, contextMap map[string]any, dlpSummary map[string]any, idem gwdomain.IdempotencyMeta, timeoutMS int, budgetRemaining *int, stageDur map[string]int64) gwdomain.RunResponse {
	latency := time.Since(start).Milliseconds()
	_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
		OrgID:                      orgID,
		ToolID:                     tool.ID,
		ToolName:                   tool.Name,
		RequestID:                  requestID,
		Actor:                      req.Actor,
		ActorRole:                  req.Role,
		ActorScopes:                req.Scopes,
		InputRedacted:              utils.Redact(input),
		ContextRedacted:            utils.Redact(contextMap),
		DLPSummary:                 dlpSummary,
		Decision:                   auditdomain.DecisionAllow,
		PolicyID:                   policyID,
		Reason:                     strPtr(reasonTemplate),
		Status:                     auditdomain.StatusError,
		ErrorCode:                  code,
		ErrorMessage:               msg,
		LatencyMS:                  int(latency),
		IdempotencyPresent:         idem.Present,
		IdempotencyOutcome:         string(idem.Outcome),
		TimeoutMS:                  intPtr(timeoutMS),
		BudgetRemainingMSAtExecute: budgetRemaining,
		StageDurationsMS:           stageDur,
	})
	s.observeRun(ctx, tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusError), time.Duration(latency)*time.Millisecond)
	return gwdomain.RunResponse{
		RequestID:   requestID,
		Decision:    gwdomain.DecisionAllow,
		ToolName:    tool.Name,
		Status:      gwdomain.RunStatusError,
		Reason:      strPtr(reason),
		ErrorCode:   code,
		ErrorMsg:    msg,
		LatencyMS:   latency,
		HTTPStatus:  httpStatus,
		Idempotency: idem,
	}
}

func (s *service) isSimEngineToolURL(rawToolURL string) bool {
	toolHostPort, ok := normalizedHostPort(rawToolURL)
	if !ok {
		return false
	}
	worldHostPort, ok := normalizedHostPort(s.cfg.SimEngineBaseURL)
	if !ok {
		return false
	}
	return strings.EqualFold(toolHostPort, worldHostPort)
}

func normalizedHostPort(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", false
		}
	}
	return host + ":" + port, true
}
