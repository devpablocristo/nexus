package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	auditdomain "nexus-gateway/internal/audit/usecases/domain"
	"nexus-gateway/internal/dlp"
	gwdomain "nexus-gateway/internal/gateway/usecases/domain"
	"nexus-gateway/internal/policy"
	policydomain "nexus-gateway/internal/policy/usecases/domain"
	secretdomain "nexus-gateway/internal/secrets/usecases/domain"
	tooldomain "nexus-gateway/internal/tool/usecases/domain"
	"nexus-gateway/pkg/types"
	"nexus-gateway/pkg/utils"
	"nexus-gateway/pkg/validations/jsonschema"
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
}

type service struct {
	toolRepo    ToolRepoPort
	policyRepo  PolicyRepoPort
	auditRepo   AuditRepoPort
	secretRepo  SecretRepoPort
	egress      EgressPort
	limiter     RateLimiterPort
	executor    HTTPExecutorPort
	idempotency IdempotencyPort
	tenantCaps  TenantLimitsPort
	metrics     RunMetricsPort
	cache       *jsonschema.CompilerCache
	evaluator   *policy.Evaluator
	dlp         *dlp.Detector
	cfg         Config
	log         zerolog.Logger
}

func NewService(toolRepo ToolRepoPort, policyRepo PolicyRepoPort, auditRepo AuditRepoPort, secretRepo SecretRepoPort, egress EgressPort, limiter RateLimiterPort, executor HTTPExecutorPort, idempotency IdempotencyPort, tenantCaps TenantLimitsPort, metrics RunMetricsPort, cache *jsonschema.CompilerCache, evaluator *policy.Evaluator, detector *dlp.Detector, cfg Config, log zerolog.Logger) Service {
	if cfg.DisableSSRFProtection {
		log.Warn().Msg("SSRF protection is DISABLED — this must only be used in test/dev environments")
	}
	return &service{
		toolRepo:    toolRepo,
		policyRepo:  policyRepo,
		auditRepo:   auditRepo,
		secretRepo:  secretRepo,
		egress:      egress,
		limiter:     limiter,
		executor:    executor,
		idempotency: idempotency,
		tenantCaps:  tenantCaps,
		metrics:     metrics,
		cache:       cache,
		evaluator:   evaluator,
		dlp:         detector,
		cfg:         cfg,
		log:         log,
	}
}

func (s *service) Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
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
	isWrite := tool.ActionType == tooldomain.ActionWrite || (tool.ActionType == "" && strings.ToUpper(tool.Method) != "GET")
	idemMeta := gwdomain.IdempotencyMeta{Present: req.IdempotencyKey != nil, Outcome: gwdomain.IdempotencySkippedNotWrite}
	var idempotencyKey string
	var requestFingerprint string
	createdIdempotencyRow := false
	if isWrite && req.IdempotencyKey != nil {
		idempotencyKey = *req.IdempotencyKey
		idemMeta.Outcome = gwdomain.IdempotencyNew
		requestFingerprint, err = buildRequestFingerprint(req.ToolName, input, req.Actor, req.Role, req.Scopes)
		if err != nil {
			return gwdomain.RunResponse{}, err
		}
		existing, err := s.idempotency.Get(ctx, orgID, tool.Name, idempotencyKey)
		if err != nil {
			return gwdomain.RunResponse{}, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store read failed")
		}
		if existing != nil {
			if existing.RequestFingerprint != requestFingerprint {
				reason := "idempotency key used with different payload"
				code := types.ErrCodeIdempotencyConflict
				idemMeta.Outcome = gwdomain.IdempotencyConflict
				return s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, reason, code, http.StatusConflict, start, input, contextMap, idemMeta, req.TimeoutMS, nil, nil), nil
			}
			switch existing.Status {
			case gwdomain.IdempotencyStatusCompleted:
				idemMeta.Outcome = gwdomain.IdempotencyReplay
				latency := time.Since(start).Milliseconds()
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
					OrgID:              orgID,
					ToolID:             tool.ID,
					ToolName:           tool.Name,
					RequestID:          requestID,
					Actor:              req.Actor,
					ActorRole:          req.Role,
					ActorScopes:        req.Scopes,
					InputRedacted:      utils.Redact(input),
					ContextRedacted:    utils.Redact(contextMap),
					DLPSummary:         map[string]any{},
					Decision:           auditdomain.Decision(decision),
					Reason:             reason,
					Status:             auditdomain.Status(status),
					OutputRedacted:     utils.Redact(result),
					ErrorCode:          errCode,
					ErrorMessage:       errMsg,
					LatencyMS:          int(latency),
					IdempotencyPresent: idemMeta.Present,
					IdempotencyOutcome: string(idemMeta.Outcome),
					TimeoutMS:          intPtr(req.TimeoutMS),
				})
				s.observeRun(ctx, tool.Name, string(decision), string(status), time.Duration(latency)*time.Millisecond)
				httpStatus := http.StatusOK
				if status == gwdomain.RunStatusError {
					httpStatus = http.StatusBadGateway
				}
				if status == gwdomain.RunStatusBlocked {
					httpStatus = http.StatusForbidden
				}
				return gwdomain.RunResponse{
					RequestID:   requestID,
					Decision:    decision,
					ToolName:    tool.Name,
					Status:      status,
					Reason:      reason,
					Result:      result,
					ErrorCode:   errCode,
					ErrorMsg:    errMsg,
					LatencyMS:   latency,
					HTTPStatus:  httpStatus,
					Idempotency: idemMeta,
				}, nil
			case gwdomain.IdempotencyStatusInProgress:
				// IMPORTANT: Stale detection enables re-execution when a previous request
				// crashed mid-flight. For write tools, this may duplicate side-effects if
				// the upstream actually completed but the gateway died before recording it.
				// Mitigation: upstream services for write tools SHOULD accept their own
				// idempotency keys to guarantee end-to-end exactly-once semantics.
				staleness := time.Duration(max(1, s.cfg.IdempotencyStalenessSeconds)) * time.Second
				if !existing.CreatedAt.IsZero() && time.Since(existing.CreatedAt) > staleness {
					_ = s.idempotency.MarkFailed(ctx, orgID, tool.Name, idempotencyKey, ptr(types.ErrCodeTimeout), map[string]any{
						"status": "error", "decision": "allow", "reason": "stale in-progress record expired",
					})
					idemMeta.Outcome = gwdomain.IdempotencyNew
					inserted, createErr := s.idempotency.CreateInProgress(ctx, gwdomain.IdempotencyRecord{
						OrgID:              orgID,
						ToolName:           tool.Name,
						IdempotencyKey:     idempotencyKey,
						RequestFingerprint: requestFingerprint,
						Status:             gwdomain.IdempotencyStatusInProgress,
						ExpiresAt:          time.Now().UTC().Add(time.Duration(max(1, s.cfg.IdempotencyTTLHours)) * time.Hour),
					})
					if createErr != nil {
						return gwdomain.RunResponse{}, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store write failed")
					}
					if !inserted {
						return gwdomain.RunResponse{}, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store write failed after stale cleanup")
					}
					createdIdempotencyRow = true
				} else {
					idemMeta.Outcome = gwdomain.IdempotencyInProgress
					reason := "idempotency request in progress"
					code := types.ErrCodeIdempotencyProgress
					return s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, reason, code, http.StatusConflict, start, input, contextMap, idemMeta, req.TimeoutMS, nil, nil), nil
				}
			case gwdomain.IdempotencyStatusFailed:
				idemMeta.Outcome = gwdomain.IdempotencyReplay
				latency := time.Since(start).Milliseconds()
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
					OrgID:              orgID,
					ToolID:             tool.ID,
					ToolName:           tool.Name,
					RequestID:          requestID,
					Actor:              req.Actor,
					ActorRole:          req.Role,
					ActorScopes:        req.Scopes,
					InputRedacted:      utils.Redact(input),
					ContextRedacted:    utils.Redact(contextMap),
					DLPSummary:         map[string]any{},
					Decision:           auditdomain.Decision(decision),
					Reason:             reason,
					Status:             auditdomain.Status(status),
					ErrorCode:          &errCode,
					ErrorMessage:       &errMsg,
					LatencyMS:          int(latency),
					IdempotencyPresent: idemMeta.Present,
					IdempotencyOutcome: string(idemMeta.Outcome),
					TimeoutMS:          intPtr(req.TimeoutMS),
				})
				s.observeRun(ctx, tool.Name, string(decision), string(status), time.Duration(latency)*time.Millisecond)
				return gwdomain.RunResponse{
					RequestID:   requestID,
					Decision:    decision,
					ToolName:    tool.Name,
					Status:      status,
					Reason:      reason,
					ErrorCode:   &errCode,
					ErrorMsg:    &errMsg,
					LatencyMS:   latency,
					HTTPStatus:  httpStatus,
					Idempotency: idemMeta,
				}, nil
			}
		} else {
			inserted, err := s.idempotency.CreateInProgress(ctx, gwdomain.IdempotencyRecord{
				OrgID:              orgID,
				ToolName:           tool.Name,
				IdempotencyKey:     idempotencyKey,
				RequestFingerprint: requestFingerprint,
				Status:             gwdomain.IdempotencyStatusInProgress,
				ExpiresAt:          time.Now().UTC().Add(time.Duration(max(1, s.cfg.IdempotencyTTLHours)) * time.Hour),
			})
			if err != nil {
				return gwdomain.RunResponse{}, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeIdempotencyStore, "idempotency store write failed")
			}
			if !inserted {
				// ON CONFLICT DO NOTHING: another request won the race
				idemMeta.Outcome = gwdomain.IdempotencyInProgress
				reason := "idempotency request in progress"
				code := types.ErrCodeIdempotencyProgress
				return s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, reason, code, http.StatusConflict, start, input, contextMap, idemMeta, req.TimeoutMS, nil, nil), nil
			}
			createdIdempotencyRow = true
		}
	}
	if !tool.Enabled {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, "tool disabled", types.ErrCodePolicyDenied, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
	}
	if tool.Kind != tooldomain.ToolKindHTTP {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, "unsupported tool kind", types.ErrCodeValidation, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
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
	if req.AuthMethod != "" {
		contextMap["auth_method"] = req.AuthMethod
	}
	dlpSummary := s.dlp.Summarize(input, contextMap)
	contextMap["dlp"] = dlpSummary
	budget.Consume("dlp", time.Since(start))

	// Input schema validation.
	schemaStart := time.Now()
	inSchema, err := s.cache.Compile(ctx, tool.ID.String()+":in", tool.InputSchemaJSON)
	if err != nil {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, "tool input schema invalid", types.ErrCodeSchemaInvalid, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
	}
	if err := jsonschema.Validate(inSchema, input); err != nil {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, "input does not match schema", types.ErrCodeValidation, http.StatusBadRequest, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
	}
	budget.Consume("schema_validation", time.Since(schemaStart))

	policies, err := s.policyRepo.ListByToolID(ctx, orgID, tool.ID)
	if err != nil {
		return gwdomain.RunResponse{}, err
	}

	match, matchReason, limits, err := s.firstMatch(policies, input, contextMap, tool)
	if err != nil {
		return gwdomain.RunResponse{}, err
	}
	if isWrite && req.IdempotencyKey == nil && limits.requireIdempotency() {
		idemMeta.Outcome = gwdomain.IdempotencyMissingRequired
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, nil, "idempotency key required by policy", types.ErrCodeIdempotencyRequired, http.StatusBadRequest, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		return resp, nil
	}

	decision := gwdomain.DecisionAllow
	var policyID *uuid.UUID
	if match != nil {
		id := match.ID
		policyID = &id
		if match.Effect == policydomain.EffectDeny {
			decision = gwdomain.DecisionDeny
		}
	} else {
		if tool.ActionType == tooldomain.ActionWrite {
			decision = gwdomain.DecisionDeny
			matchReason = "default deny for write tool"
		} else {
			matchReason = "default allow for read tool"
		}
	}

	// Enforce max_bytes_input/context.
	if decision == gwdomain.DecisionAllow {
		if maxIn := limits.maxBytesInput(s.cfg.MaxBytesInputDefault); maxIn > 0 {
			n, err := utils.JSONSize(input)
			if err != nil {
				return gwdomain.RunResponse{}, err
			}
			if n > maxIn {
				resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, "input too large", types.ErrCodePolicyDenied, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
				s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
				return resp, nil
			}
		}
		if maxCtx := limits.maxBytesContext(s.cfg.MaxBytesContextDefault); maxCtx > 0 {
			n, err := utils.JSONSize(contextMap)
			if err != nil {
				return gwdomain.RunResponse{}, err
			}
			if n > maxCtx {
				resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, "context too large", types.ErrCodePolicyDenied, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
				s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
				return resp, nil
			}
		}
	}

	if decision == gwdomain.DecisionDeny {
		reason := matchReason
		code := types.ErrCodePolicyDenied
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, reason, code, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
	}

	// Tenant-level hard limit (plan cap)
	tenantRunRPM := 0
	if s.tenantCaps != nil {
		tenantRunRPM, err = s.tenantCaps.GetRunRPM(ctx, orgID)
		if err != nil {
			return gwdomain.RunResponse{}, err
		}
		if tenantRunRPM > 0 {
			tenantKey := orgID.String() + ":tenant"
			if !s.limiter.Allow(tenantKey, tenantRunRPM) {
				resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, "tenant run rate limit exceeded", types.ErrCodeRateLimited, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
				s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
				return resp, nil
			}
		}
	}

	// Rate limit
	perMin := limits.rateLimitPerMinute(s.cfg.DefaultRateLimitPerMinute)
	if perMin > 0 {
		key := orgID.String() + ":" + tool.Name
		if !s.limiter.Allow(key, perMin) {
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, "rate limit exceeded", types.ErrCodeRateLimited, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
			s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
			return resp, nil
		}
	}

	u, parseErr := url.Parse(tool.URL)
	if parseErr != nil || u.Hostname() == "" {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, "invalid tool url", types.ErrCodeValidation, http.StatusBadRequest, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
	}

	if !s.cfg.DisableSSRFProtection {
		if err := utils.ValidateEgressURL(tool.URL); err != nil {
			resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, "ssrf blocked: "+err.Error(), types.ErrCodeEgressDenied, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
			s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
			return resp, nil
		}
	}
	allowed, err := s.egress.IsHostAllowed(ctx, orgID, tool.ID, strings.ToLower(u.Hostname()))
	if err != nil {
		return gwdomain.RunResponse{}, err
	}
	if !allowed {
		resp := s.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, requestID, tool.Name, tool.ID, policyID, "egress host denied", types.ErrCodeEgressDenied, http.StatusForbidden, start, input, contextMap, idemMeta, req.TimeoutMS, intPtr(budget.RemainingMS()), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
	}

	headers := map[string]string{}
	secrets, err := s.secretRepo.ListForTool(ctx, orgID, tool.ID)
	if err != nil {
		return gwdomain.RunResponse{}, err
	}
	for _, secret := range secrets {
		if !secret.Enabled {
			continue
		}
		if strings.EqualFold(secret.SecretType, "header") && secret.KeyName != "" {
			headers[secret.KeyName] = secret.PlaintextValue
		}
		if strings.EqualFold(secret.SecretType, "bearer") {
			headers["Authorization"] = "Bearer " + secret.PlaintextValue
		}
	}

	remainingBeforeExec := budget.RemainingMS()
	if remainingBeforeExec <= 0 {
		code := types.ErrCodeTimeoutBudget
		reason := "timeout budget exhausted before execute"
		resp := s.errorRun(ctx, orgID, req, tool, requestID, policyID, matchReason, reason, &code, &reason, http.StatusRequestTimeout, start, input, contextMap, dlpSummary, idemMeta, req.TimeoutMS, intPtr(0), budget.StageDurationsMS())
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
		return resp, nil
	}
	execTimeoutMS := remainingBeforeExec
	execCtx, cancelExec := context.WithTimeout(ctx, time.Duration(execTimeoutMS)*time.Millisecond)
	execStart := time.Now()
	maxRetries := s.cfg.HTTPRetries
	if isWrite {
		maxRetries = 0
	}
	result, _, he := s.executor.Execute(execCtx, tool.Method, tool.URL, input, headers, maxRetries)
	cancelExec()
	budget.Consume("execute_http", time.Since(execStart))
	latency := time.Since(start).Milliseconds()

	// Output schema validation (best-effort).
	if he == nil && len(tool.OutputSchemaJSON) > 0 {
		outSchema, err := s.cache.Compile(ctx, tool.ID.String()+":out", tool.OutputSchemaJSON)
		if err != nil || jsonschema.Validate(outSchema, result) != nil {
			code := types.ErrCodeOutputSchemaInvalid
			msg := "tool output does not match schema"
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
				Reason:                     strPtr(matchReason),
				Status:                     auditdomain.StatusError,
				OutputRedacted:             utils.Redact(result),
				ErrorCode:                  &code,
				ErrorMessage:               &msg,
				LatencyMS:                  int(latency),
				IdempotencyPresent:         idemMeta.Present,
				IdempotencyOutcome:         string(idemMeta.Outcome),
				TimeoutMS:                  intPtr(req.TimeoutMS),
				BudgetRemainingMSAtExecute: intPtr(remainingBeforeExec),
				StageDurationsMS:           budget.StageDurationsMS(),
			})
			resp := gwdomain.RunResponse{Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, HTTPStatus: http.StatusBadGateway}
			s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &resp)
			s.observeRun(ctx, tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusError), time.Duration(latency)*time.Millisecond)
			return gwdomain.RunResponse{
				RequestID:   requestID,
				Decision:    gwdomain.DecisionAllow,
				ToolName:    tool.Name,
				Status:      gwdomain.RunStatusError,
				ErrorCode:   &code,
				ErrorMsg:    &msg,
				LatencyMS:   latency,
				HTTPStatus:  http.StatusBadGateway,
				Idempotency: idemMeta,
			}, nil
		}
	}

	if he != nil {
		code := he.Code
		if he.Code == types.ErrCodeTimeout && budget.RemainingMS() <= 0 {
			code = types.ErrCodeTimeoutBudget
		}
		msg := he.Message
		s.log.Error().
			Str("request_id", requestID).
			Str("org_id", orgID.String()).
			Str("tool_name", tool.Name).
			Str("decision", "allow").
			Str("status", "error").
			Str("error_code", code).
			Msg("run_error")
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
			Reason:                     strPtr(matchReason),
			Status:                     auditdomain.StatusError,
			OutputRedacted:             nil,
			ErrorCode:                  &code,
			ErrorMessage:               &msg,
			LatencyMS:                  int(latency),
			IdempotencyPresent:         idemMeta.Present,
			IdempotencyOutcome:         string(idemMeta.Outcome),
			TimeoutMS:                  intPtr(req.TimeoutMS),
			BudgetRemainingMSAtExecute: intPtr(remainingBeforeExec),
			StageDurationsMS:           budget.StageDurationsMS(),
		})
		status := http.StatusBadGateway
		if he.Code == types.ErrCodeInvalidGETInput || he.Code == types.ErrCodeValidation {
			status = http.StatusBadRequest
		}
		if code == types.ErrCodeTimeoutBudget {
			status = http.StatusRequestTimeout
		}
		respForIdem := gwdomain.RunResponse{Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusError, ErrorCode: &code, ErrorMsg: &msg, HTTPStatus: status}
		s.failIdempotencyIfNeeded(ctx, createdIdempotencyRow, orgID, tool.Name, idempotencyKey, &respForIdem)
		s.observeRun(ctx, tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusError), time.Duration(latency)*time.Millisecond)
		return gwdomain.RunResponse{
			RequestID:   requestID,
			Decision:    gwdomain.DecisionAllow,
			ToolName:    tool.Name,
			Status:      gwdomain.RunStatusError,
			ErrorCode:   &code,
			ErrorMsg:    &msg,
			LatencyMS:   latency,
			HTTPStatus:  status,
			Idempotency: idemMeta,
		}, nil
	}

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
		Reason:                     strPtr(matchReason),
		Status:                     auditdomain.StatusSuccess,
		OutputRedacted:             utils.Redact(result),
		LatencyMS:                  int(latency),
		IdempotencyPresent:         idemMeta.Present,
		IdempotencyOutcome:         string(idemMeta.Outcome),
		TimeoutMS:                  intPtr(req.TimeoutMS),
		BudgetRemainingMSAtExecute: intPtr(remainingBeforeExec),
		StageDurationsMS:           budget.StageDurationsMS(),
	})
	if createdIdempotencyRow {
		_ = s.idempotency.MarkCompleted(ctx, orgID, tool.Name, idempotencyKey, map[string]any{
			"decision": string(gwdomain.DecisionAllow),
			"status":   string(gwdomain.RunStatusSuccess),
			"result":   utils.Redact(result),
		})
	}

	s.log.Info().
		Str("request_id", requestID).
		Str("org_id", orgID.String()).
		Str("tool_name", tool.Name).
		Str("decision", "allow").
		Str("status", "success").
		Int64("latency_ms", latency).
		Msg("run_success")
	s.observeRun(ctx, tool.Name, string(gwdomain.DecisionAllow), string(gwdomain.RunStatusSuccess), time.Duration(latency)*time.Millisecond)
	return gwdomain.RunResponse{
		RequestID:   requestID,
		Decision:    gwdomain.DecisionAllow,
		ToolName:    tool.Name,
		Status:      gwdomain.RunStatusSuccess,
		Result:      result,
		LatencyMS:   latency,
		HTTPStatus:  http.StatusOK,
		Idempotency: idemMeta,
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
