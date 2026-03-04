package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	"nexus/pkg/types"
	"nexus/pkg/utils"
	"nexus/pkg/validations/jsonschema"
)

type ToolRepoPort interface {
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
	GetByID(ctx context.Context, orgID, toolID uuid.UUID) (tooldomain.Tool, error)
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

type ApprovalPort interface {
	RequestApproval(ctx context.Context, req ApprovalRequest) (string, error)
}

type ApprovalRequest struct {
	OrgID           uuid.UUID
	ToolID          uuid.UUID
	RequestID       string
	ToolName        string
	Actor           *string
	Role            *string
	InputRedacted   map[string]any
	ContextRedacted map[string]any
	Reason          string
	PolicyID        *uuid.UUID
	TTLSeconds      int
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
	tenantRunRPM          int
	headers               map[string]string
	remainingBeforeExec   int
	latency               int64
	result                any
	execErr               *types.HTTPError
}

type Usecases struct {
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
	approval        ApprovalPort
	metrics         RunMetricsPort
	cache           *jsonschema.CompilerCache
	evaluator       *policy.Evaluator
	dlp             *dlp.Detector
	cfg             Config
	log             zerolog.Logger
}

func NewUsecases(toolRepo ToolRepoPort, policyRepo PolicyRepoPort, auditRepo AuditRepoPort, secretRepo SecretRepoPort, egress EgressPort, limiter RateLimiterPort, executor HTTPExecutorPort, idempotency IdempotencyPort, tenantCaps TenantLimitsPort, actionOverrides ActionOverridesPort, approval ApprovalPort, metrics RunMetricsPort, cache *jsonschema.CompilerCache, evaluator *policy.Evaluator, detector *dlp.Detector, cfg Config, log zerolog.Logger) *Usecases {
	if cfg.DisableSSRFProtection {
		log.Warn().Msg("SSRF protection is DISABLED — this must only be used in test/dev environments")
	}
	return &Usecases{
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
		approval:        approval,
		metrics:         metrics,
		cache:           cache,
		evaluator:       evaluator,
		dlp:             detector,
		cfg:             cfg,
		log:             log,
	}
}

func (u *Usecases) Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	// 1. Inicialización
	start := time.Now()
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.NewString()
	}
	budget := gwdomain.NewTimeoutBudget(gwdomain.ClampTimeoutMS(req.TimeoutMS, u.cfg.TimeoutBudgetDefaultMS, u.cfg.TimeoutBudgetMinMS, u.cfg.TimeoutBudgetMaxMS))
	req.TimeoutMS = budget.TimeoutMS()

	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	contextMap := req.Context
	if contextMap == nil {
		contextMap = map[string]any{}
	}

	// 2. Resolución de tool (by tool_id if provided, otherwise by tool_name)
	tool, err := u.resolveTool(ctx, orgID, req.ToolID, req.ToolName)
	if err != nil {
		toolRef := req.ToolName
		if toolRef == "" {
			toolRef = req.ToolID
		}
		var he types.HTTPError
		if errors.As(err, &he) && he.Code == types.ErrCodeNotFound {
			reason := "tool not found"
			code := types.ErrCodeNotFound
			latency := time.Since(start).Milliseconds()
			u.observeRun(ctx, toolRef, string(gwdomain.DecisionDeny), string(gwdomain.RunStatusBlocked), time.Duration(latency)*time.Millisecond)
			u.log.Warn().
				Str("request_id", requestID).
				Str("org_id", orgID.String()).
				Str("tool_ref", toolRef).
				Str("decision", "deny").
				Str("status", "blocked").
				Int64("latency_ms", latency).
				Str("error_code", code).
				Msg("run_blocked")
			return gwdomain.RunResponse{
				RequestID:  requestID,
				Decision:   gwdomain.DecisionDeny,
				ToolName:   toolRef,
				Status:     gwdomain.RunStatusBlocked,
				Reason:     &reason,
				ErrorCode:  &code,
				ErrorMsg:   &reason,
				LatencyMS:  latency,
				HTTPStatus: http.StatusNotFound,
			}, nil
		}
		u.log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("org_id", orgID.String()).
			Str("tool_ref", toolRef).
			Msg("tool_lookup_failed")
		return gwdomain.RunResponse{}, err
	}
	req.ToolName = tool.Name

	// 3. Idempotencia
	isWrite := tool.ActionType == tooldomain.ActionWrite || (tool.ActionType == "" && strings.ToUpper(tool.Method) != "GET")
	idemMeta := gwdomain.IdempotencyMeta{Present: req.IdempotencyKey != nil, Outcome: gwdomain.IdempotencySkippedNotWrite}
	st := &runState{
		start: start, requestID: requestID, budget: budget, input: input, contextMap: contextMap,
		tool: tool, isWrite: isWrite, idemMeta: idemMeta,
	}
	if resp, err := u.runResolveIdempotency(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}

	// 4-7. Validación tool, contexto, DLP, schema entrada
	if resp, err := u.runValidateAndPrepare(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}
	// 8-9. Políticas y decisión allow/deny
	if resp, err := u.runPoliciesAndDecision(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}
	// 10-14. Action overrides, rate limits, URL/egress, secretos
	if resp, err := u.runOverridesRateLimitsEgressSecrets(ctx, orgID, req, st); err != nil {
		return gwdomain.RunResponse{}, err
	} else if resp != nil {
		return *resp, nil
	}
	// 15-19. Timeout, ejecución HTTP, schema salida, auditoría, respuesta
	return u.runExecuteAndFinish(ctx, orgID, req, st)
}

// runResolveIdempotency implementa:
//  3. Idempotencia
func (u *Usecases) runResolveIdempotency(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
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
	existing, err := u.idempotency.Get(ctx, orgID, st.tool.Name, st.idempotencyKey)
	if err != nil {
		return nil, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store read failed")
	}
	if existing != nil {
		if existing.RequestFingerprint != st.requestFingerprint {
			reason := "idempotency key used with different payload"
			code := types.ErrCodeIdempotencyConflict
			st.idemMeta.Outcome = gwdomain.IdempotencyConflict
			resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, reason, code, http.StatusConflict, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, nil, nil)
			return &resp, nil
		}
		switch existing.Status {
		case gwdomain.IdempotencyStatusCompleted:
			return u.handleIdempotencyReplayCompleted(ctx, orgID, req, st, existing), nil
		case gwdomain.IdempotencyStatusInProgress:
			staleness := time.Duration(max(1, u.cfg.IdempotencyStalenessSeconds)) * time.Second
			if !existing.CreatedAt.IsZero() && time.Since(existing.CreatedAt) > staleness {
				_ = u.idempotency.MarkFailed(ctx, orgID, st.tool.Name, st.idempotencyKey, ptr(types.ErrCodeTimeout), map[string]any{
					"status": "error", "decision": "allow", "reason": "stale in-progress record expired",
				})
				st.idemMeta.Outcome = gwdomain.IdempotencyNew
				inserted, createErr := u.idempotency.CreateInProgress(ctx, gwdomain.IdempotencyRecord{
					OrgID: orgID, ToolName: st.tool.Name, IdempotencyKey: st.idempotencyKey,
					RequestFingerprint: st.requestFingerprint, Status: gwdomain.IdempotencyStatusInProgress,
					ExpiresAt: time.Now().UTC().Add(time.Duration(max(1, u.cfg.IdempotencyTTLHours)) * time.Hour),
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
				resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, reason, code, http.StatusConflict, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, nil, nil)
				return &resp, nil
			}
		case gwdomain.IdempotencyStatusFailed:
			return u.handleIdempotencyReplayFailed(ctx, orgID, req, st, existing), nil
		}
	} else {
		inserted, err := u.idempotency.CreateInProgress(ctx, gwdomain.IdempotencyRecord{
			OrgID: orgID, ToolName: st.tool.Name, IdempotencyKey: st.idempotencyKey,
			RequestFingerprint: st.requestFingerprint, Status: gwdomain.IdempotencyStatusInProgress,
			ExpiresAt: time.Now().UTC().Add(time.Duration(max(1, u.cfg.IdempotencyTTLHours)) * time.Hour),
		})
		if err != nil {
			return nil, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store write failed")
		}
		if !inserted {
			st.idemMeta.Outcome = gwdomain.IdempotencyInProgress
			reason := "idempotency request in progress"
			code := types.ErrCodeIdempotencyProgress
			resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, reason, code, http.StatusConflict, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, nil, nil)
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
func (u *Usecases) runValidateAndPrepare(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	// 4. Validación tool (enabled, kind)
	if !st.tool.Enabled {
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "tool disabled", types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	if st.tool.Kind != tooldomain.ToolKindHTTP {
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "unsupported tool kind", types.ErrCodeValidation, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
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
	st.dlpSummary = u.dlp.Summarize(st.input, st.contextMap)
	st.contextMap["dlp"] = st.dlpSummary
	st.budget.Consume("dlp", time.Since(st.start))

	// 7. Validación schema de entrada
	schemaStart := time.Now()
	inSchema, err := u.cache.Compile(ctx, st.tool.ID.String()+":in", st.tool.InputSchemaJSON)
	if err != nil {
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "tool input schema invalid", types.ErrCodeSchemaInvalid, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	if err := jsonschema.Validate(inSchema, st.input); err != nil {
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "input does not match schema", types.ErrCodeValidation, http.StatusBadRequest, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	st.budget.Consume("schema_validation", time.Since(schemaStart))
	return nil, nil
}

// runPoliciesAndDecision implementa:
//  8. Políticas y decisión allow/deny (firstMatch, límites input/context)
//  9. Aplicar decisión deny
func (u *Usecases) runPoliciesAndDecision(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	// 8. Políticas y decisión allow/deny (firstMatch, límites input/context)
	policies, err := u.policyRepo.ListByToolID(ctx, orgID, st.tool.ID)
	if err != nil {
		return nil, err
	}
	match, matchReason, limits, err := u.firstMatch(policies, st.input, st.contextMap, st.tool)
	if err != nil {
		return nil, err
	}
	st.matchReason = matchReason
	st.limits = limits
	if st.isWrite && req.IdempotencyKey == nil && limits.requireIdempotency() {
		st.idemMeta.Outcome = gwdomain.IdempotencyMissingRequired
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, nil, "idempotency key required by policy", types.ErrCodeIdempotencyRequired, http.StatusBadRequest, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
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
		if maxIn := limits.maxBytesInput(u.cfg.MaxBytesInputDefault); maxIn > 0 {
			n, err := utils.JSONSize(st.input)
			if err != nil {
				return nil, err
			}
			if n > maxIn {
				resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, policyID, "input too large", types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
				u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
				return &resp, nil
			}
		}
		if maxCtx := limits.maxBytesContext(u.cfg.MaxBytesContextDefault); maxCtx > 0 {
			n, err := utils.JSONSize(st.contextMap)
			if err != nil {
				return nil, err
			}
			if n > maxCtx {
				resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, policyID, "context too large", types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
				u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
				return &resp, nil
			}
		}
	}
	// 9. Aplicar decisión deny
	if decision == gwdomain.DecisionDeny {
		reason := st.matchReason
		code := types.ErrCodePolicyDenied
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, reason, code, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	// 9b. Approval check: policy requires human approval before execution
	if st.limits.requireApproval() && u.approval != nil {
		approvalID, err := u.approval.RequestApproval(ctx, ApprovalRequest{
			OrgID:           orgID,
			ToolID:          st.tool.ID,
			RequestID:       st.requestID,
			ToolName:        st.tool.Name,
			Actor:           req.Actor,
			Role:            req.Role,
			InputRedacted:   redactToMap(st.input),
			ContextRedacted: redactToMap(st.contextMap),
			Reason:          st.matchReason,
			PolicyID:        st.policyID,
			TTLSeconds:      st.limits.approvalTTLSeconds(),
		})
		if err != nil {
			u.log.Error().Err(err).Str("request_id", st.requestID).Msg("approval_request_failed")
			return nil, err
		}
		reason := "pending human approval (id: " + approvalID + ")"
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, reason, types.ErrCodeApprovalRequired, http.StatusAccepted, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	return nil, nil
}

// runOverridesRateLimitsEgressSecrets implementa:
// 10. Action overrides, 11. Rate limit tenant, 12. Rate limit tool,
// 13. URL y egress (SSRF, allowlist), 14. Secretos (headers/bearer)
func (u *Usecases) runOverridesRateLimitsEgressSecrets(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	if resp, err := u.checkActionOverridesAndTenantCaps(ctx, orgID, req, st); err != nil || resp != nil {
		return resp, err
	}
	if resp, err := u.checkTenantRateLimit(ctx, orgID, req, st); err != nil || resp != nil {
		return resp, err
	}
	if resp, err := u.checkToolRateLimit(ctx, orgID, req, st); err != nil || resp != nil {
		return resp, err
	}
	if resp, err := u.validateURLAndEgress(ctx, orgID, req, st, st.tool); err != nil || resp != nil {
		return resp, err
	}
	return nil, u.resolveSecrets(ctx, orgID, st)
}

// runExecuteAndFinish implementa:
// 15. Timeout budget antes de ejecutar
// 16. Ejecución HTTP al upstream
// 17. Validación schema de salida (best-effort)
// 18. Manejo error de ejecución
// 19. Éxito: auditoría, idempotency completed, respuesta
func (u *Usecases) runExecuteAndFinish(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (gwdomain.RunResponse, error) {
	// 15. Timeout budget antes de ejecutar
	st.remainingBeforeExec = st.budget.RemainingMS()
	if st.remainingBeforeExec <= 0 {
		code := types.ErrCodeTimeoutBudget
		reason := "timeout budget exhausted before execute"
		resp := u.errorRun(ctx, orgID, req, st.tool, st.requestID, st.policyID, st.matchReason, reason, &code, &reason, http.StatusRequestTimeout, st.start, st.input, st.contextMap, st.dlpSummary, st.idemMeta, req.TimeoutMS, intPtr(0), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
		return resp, nil
	}
	execTimeoutMS := st.remainingBeforeExec
	execCtx, cancelExec := context.WithTimeout(ctx, time.Duration(execTimeoutMS)*time.Millisecond)
	execStart := time.Now()
	maxRetries := u.cfg.HTTPRetries
	if st.isWrite {
		maxRetries = 0
	}
	// 16. Ejecución HTTP al upstream
	st.result, _, st.execErr = u.executor.Execute(execCtx, st.tool.Method, st.tool.URL, st.input, st.headers, maxRetries)
	cancelExec()
	st.budget.Consume("execute_http", time.Since(execStart))
	st.latency = time.Since(st.start).Milliseconds()

	// 17. Validación schema de salida (best-effort)
	if resp, ok := u.validateOutputSchema(ctx, orgID, req, st); ok {
		return resp, nil
	}
	// 18. Manejo error de ejecución
	if resp, ok := u.handleExecError(ctx, orgID, req, st); ok {
		return resp, nil
	}
	// 19. Éxito: auditoría, idempotency completed, respuesta
	return u.auditSuccessAndComplete(ctx, orgID, req, st, st.tool), nil
}

func (u *Usecases) Simulate(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.SimulateResponse, error) {
	st := u.initSimulateState(req)

	tool, err := u.resolveTool(ctx, orgID, req.ToolID, req.ToolName)
	if err != nil {
		toolRef := req.ToolName
		if toolRef == "" {
			toolRef = req.ToolID
		}
		var he types.HTTPError
		if errors.As(err, &he) && he.Code == types.ErrCodeNotFound {
			st.explain["stage"] = "tool_lookup"
			st.explain["result"] = "not_found"
			return u.simulateDeny(st, tooldomain.Tool{Name: toolRef}, "tool not found", types.ErrCodeNotFound, http.StatusNotFound, ""), nil
		}
		return gwdomain.SimulateResponse{}, err
	}
	req.ToolName = tool.Name
	st.explain["tool_id"] = tool.ID.String()
	st.explain["tool_name"] = tool.Name

	if !tool.Enabled {
		return u.simulateDeny(st, tool, "tool disabled", types.ErrCodePolicyDenied, http.StatusForbidden, "tool_enabled"), nil
	}

	u.simulateEnrichContext(req, &st)
	dlpSummary := u.dlp.Summarize(st.input, st.contextMap)
	st.contextMap["dlp"] = dlpSummary
	st.explain["dlp_summary"] = dlpSummary

	if resp := u.simulateSchemaCheck(ctx, &st, tool); resp != nil {
		return *resp, nil
	}

	decision, matchReason, policyID := u.simulatePoliciesAndLimits(ctx, orgID, &st, tool)

	allowed, err := u.simulateEgressCheck(ctx, orgID, &st, tool)
	if err != nil {
		return gwdomain.SimulateResponse{}, err
	}
	if !allowed {
		decision = gwdomain.DecisionDeny
		matchReason = "egress host denied"
	}

	secrets, err := u.secretRepo.ListForTool(ctx, orgID, tool.ID)
	if err != nil {
		return gwdomain.SimulateResponse{}, err
	}
	st.explain["secret_count"] = len(secrets)
	st.explain["rate_limit_checked"] = false
	st.explain["would_execute"] = decision == gwdomain.DecisionAllow
	st.explain["policy_id"] = ""
	if policyID != nil {
		st.explain["policy_id"] = policyID.String()
	}

	if decision == gwdomain.DecisionDeny {
		code := types.ErrCodePolicyDenied
		if matchReason == "egress host denied" {
			code = types.ErrCodeEgressDenied
		}
		return u.simulateDeny(st, tool, matchReason, code, http.StatusForbidden, ""), nil
	}
	return u.simulateAllow(st, tool, matchReason), nil
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

func redactToMap(m map[string]any) map[string]any {
	v := utils.Redact(m)
	if out, ok := v.(map[string]any); ok {
		return out
	}
	return m
}

func (l parsedLimits) requireApproval() bool {
	if l.m == nil {
		return false
	}
	v, ok := l.m["require_approval"]
	if !ok {
		return false
	}
	if t, ok := v.(bool); ok {
		return t
	}
	return false
}

func (l parsedLimits) approvalTTLSeconds() int {
	if l.m == nil {
		return 3600
	}
	if v, ok := l.m["approval_ttl_seconds"].(float64); ok && v > 0 {
		return int(v)
	}
	return 3600
}

func (u *Usecases) firstMatch(policies []policydomain.Policy, input, contextMap map[string]any, tool tooldomain.Tool) (*policydomain.Policy, string, parsedLimits, error) {
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
		ok, err := u.evaluator.Matches(p.ConditionsJSON, input, contextMap, attrs)
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

// resolveTool resolves a tool by ID (if provided) or by name. tool_id takes precedence.
func (u *Usecases) resolveTool(ctx context.Context, orgID uuid.UUID, toolID, toolName string) (tooldomain.Tool, error) {
	if toolID != "" {
		id, err := uuid.Parse(toolID)
		if err != nil {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "tool_id is not a valid UUID")
		}
		return u.toolRepo.GetByID(ctx, orgID, id)
	}
	if toolName == "" {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "tool_name or tool_id required")
	}
	return u.toolRepo.GetByName(ctx, orgID, toolName)
}

func (u *Usecases) blocked(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, requestID string, toolName string, toolIDVal uuid.UUID, policyID *uuid.UUID, reason string, code string, httpStatus int, start time.Time, input, contextMap map[string]any, idem gwdomain.IdempotencyMeta, timeoutMS int, budgetRemaining *int, stageDur map[string]int64) gwdomain.RunResponse {
	latency := time.Since(start).Milliseconds()
	decision := auditdomain.DecisionDeny
	status := auditdomain.StatusBlocked
	rc := reason
	_ = u.auditRepo.Create(ctx, auditdomain.AuditEvent{
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
	u.log.Warn().
		Str("request_id", requestID).
		Str("org_id", orgID.String()).
		Str("tool_name", toolName).
		Str("decision", "deny").
		Str("status", "blocked").
		Int64("latency_ms", latency).
		Str("error_code", code).
		Msg("run_blocked")
	annotateRunSpan(ctx, orgID, input, toolName, requestID, "deny", policyID)
	u.observeRun(ctx, toolName, string(gwdomain.DecisionDeny), string(gwdomain.RunStatusBlocked), time.Duration(latency)*time.Millisecond)
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

func (u *Usecases) observeRun(ctx context.Context, toolName, decision, status string, latency time.Duration) {
	if u.metrics == nil {
		return
	}
	u.metrics.ObserveRun(ctx, toolName, decision, status, latency)
}

func (u *Usecases) failIdempotencyIfNeeded(ctx context.Context, shouldUpdate bool, orgID uuid.UUID, toolName, key string, resp *gwdomain.RunResponse) {
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
	_ = u.idempotency.MarkFailed(ctx, orgID, toolName, key, code, response)
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

func (u *Usecases) errorRun(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, tool tooldomain.Tool, requestID string, policyID *uuid.UUID, reasonTemplate, reason string, code, msg *string, httpStatus int, start time.Time, input, contextMap map[string]any, dlpSummary map[string]any, idem gwdomain.IdempotencyMeta, timeoutMS int, budgetRemaining *int, stageDur map[string]int64) gwdomain.RunResponse {
	latency := time.Since(start).Milliseconds()
	_ = u.auditRepo.Create(ctx, auditdomain.AuditEvent{
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
	u.observeRun(ctx, tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusError), time.Duration(latency)*time.Millisecond)
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

