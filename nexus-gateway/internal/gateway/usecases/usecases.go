package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	auditdomain "nexus-gateway/internal/audit/usecases/domain"
	gwdomain "nexus-gateway/internal/gateway/usecases/domain"
	policyuc "nexus-gateway/internal/policy/usecases"
	policydomain "nexus-gateway/internal/policy/usecases/domain"
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

type RateLimiterPort interface {
	Allow(key string, perMinute int) bool
}

type HTTPExecutorPort interface {
	Execute(ctx context.Context, method, url string, input map[string]any) (any, int, *types.HTTPError)
}

type Service interface {
	Run(ctx context.Context, orgID uuid.UUID, actor *string, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type Config struct {
	DefaultRateLimitPerMinute int
	MaxBytesInputDefault      int
	MaxBytesContextDefault    int
}

type service struct {
	toolRepo   ToolRepoPort
	policyRepo PolicyRepoPort
	auditRepo  AuditRepoPort
	limiter    RateLimiterPort
	executor   HTTPExecutorPort
	cache      *jsonschema.CompilerCache
	evaluator  *policyuc.Evaluator
	cfg        Config
	log        zerolog.Logger
}

func NewService(toolRepo ToolRepoPort, policyRepo PolicyRepoPort, auditRepo AuditRepoPort, limiter RateLimiterPort, executor HTTPExecutorPort, cache *jsonschema.CompilerCache, evaluator *policyuc.Evaluator, cfg Config, log zerolog.Logger) Service {
	return &service{
		toolRepo:   toolRepo,
		policyRepo: policyRepo,
		auditRepo:  auditRepo,
		limiter:    limiter,
		executor:   executor,
		cache:      cache,
		evaluator:  evaluator,
		cfg:        cfg,
		log:        log,
	}
}

func (s *service) Run(ctx context.Context, orgID uuid.UUID, actor *string, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
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

	tool, err := s.toolRepo.GetByName(ctx, orgID, req.ToolName)
	if err != nil {
		var he types.HTTPError
		if errors.As(err, &he) && he.Code == types.ErrCodeNotFound {
			// Can't write audit without a valid tool_id due to FK constraints.
			reason := "tool not found"
			code := types.ErrCodeNotFound
			latency := time.Since(start).Milliseconds()
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
	if !tool.Enabled {
		return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, nil, "tool disabled", types.ErrCodePolicyDenied, http.StatusForbidden, start, input, contextMap), nil
	}
	if tool.Kind != tooldomain.ToolKindHTTP {
		return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, nil, "unsupported tool kind", types.ErrCodeValidation, http.StatusForbidden, start, input, contextMap), nil
	}

	// Input schema validation.
	inSchema, err := s.cache.Compile(ctx, tool.ID.String()+":in", tool.InputSchemaJSON)
	if err != nil {
		return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, nil, "tool input schema invalid", types.ErrCodeSchemaInvalid, http.StatusForbidden, start, input, contextMap), nil
	}
	if err := jsonschema.Validate(inSchema, input); err != nil {
		return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, nil, "input does not match schema", types.ErrCodeValidation, http.StatusBadRequest, start, input, contextMap), nil
	}

	policies, err := s.policyRepo.ListByToolID(ctx, orgID, tool.ID)
	if err != nil {
		return gwdomain.RunResponse{}, err
	}

	match, matchReason, limits, err := s.firstMatch(policies, input, contextMap, tool)
	if err != nil {
		return gwdomain.RunResponse{}, err
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
				return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, policyID, "input too large", types.ErrCodePolicyDenied, http.StatusForbidden, start, input, contextMap), nil
			}
		}
		if maxCtx := limits.maxBytesContext(s.cfg.MaxBytesContextDefault); maxCtx > 0 {
			n, err := utils.JSONSize(contextMap)
			if err != nil {
				return gwdomain.RunResponse{}, err
			}
			if n > maxCtx {
				return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, policyID, "context too large", types.ErrCodePolicyDenied, http.StatusForbidden, start, input, contextMap), nil
			}
		}
	}

	if decision == gwdomain.DecisionDeny {
		reason := matchReason
		code := types.ErrCodePolicyDenied
		return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, policyID, reason, code, http.StatusForbidden, start, input, contextMap), nil
	}

	// Rate limit
	perMin := limits.rateLimitPerMinute(s.cfg.DefaultRateLimitPerMinute)
	if perMin > 0 {
		key := orgID.String() + ":" + tool.Name
		if !s.limiter.Allow(key, perMin) {
			return s.blocked(ctx, orgID, actor, requestID, tool.Name, tool.ID, policyID, "rate limit exceeded", types.ErrCodeRateLimited, http.StatusForbidden, start, input, contextMap), nil
		}
	}

	result, _, he := s.executor.Execute(ctx, tool.Method, tool.URL, input)
	latency := time.Since(start).Milliseconds()

	// Output schema validation (best-effort).
	if he == nil && len(tool.OutputSchemaJSON) > 0 {
		outSchema, err := s.cache.Compile(ctx, tool.ID.String()+":out", tool.OutputSchemaJSON)
		if err != nil || jsonschema.Validate(outSchema, result) != nil {
			code := types.ErrCodeOutputSchemaInvalid
			msg := "tool output does not match schema"
			_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
				OrgID:           orgID,
				ToolID:          tool.ID,
				ToolName:        tool.Name,
				RequestID:       requestID,
				Actor:           actor,
				InputRedacted:   utils.Redact(input),
				ContextRedacted: utils.Redact(contextMap),
				Decision:        auditdomain.DecisionAllow,
				PolicyID:        policyID,
				Reason:          strPtr(matchReason),
				Status:          auditdomain.StatusError,
				OutputRedacted:  utils.Redact(result),
				ErrorCode:       &code,
				ErrorMessage:    &msg,
				LatencyMS:       int(latency),
			})
			return gwdomain.RunResponse{
				RequestID:  requestID,
				Decision:   gwdomain.DecisionAllow,
				ToolName:   tool.Name,
				Status:     gwdomain.RunStatusError,
				ErrorCode:  &code,
				ErrorMsg:   &msg,
				LatencyMS:  latency,
				HTTPStatus: http.StatusBadGateway,
			}, nil
		}
	}

	if he != nil {
		code := he.Code
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
			OrgID:           orgID,
			ToolID:          tool.ID,
			ToolName:        tool.Name,
			RequestID:       requestID,
			Actor:           actor,
			InputRedacted:   utils.Redact(input),
			ContextRedacted: utils.Redact(contextMap),
			Decision:        auditdomain.DecisionAllow,
			PolicyID:        policyID,
			Reason:          strPtr(matchReason),
			Status:          auditdomain.StatusError,
			OutputRedacted:  nil,
			ErrorCode:       &code,
			ErrorMessage:    &msg,
			LatencyMS:       int(latency),
		})
		status := http.StatusBadGateway
		if he.Code == types.ErrCodeInvalidGETInput || he.Code == types.ErrCodeValidation {
			status = http.StatusBadRequest
		}
		return gwdomain.RunResponse{
			RequestID:  requestID,
			Decision:   gwdomain.DecisionAllow,
			ToolName:   tool.Name,
			Status:     gwdomain.RunStatusError,
			ErrorCode:  &code,
			ErrorMsg:   &msg,
			LatencyMS:  latency,
			HTTPStatus: status,
		}, nil
	}

	_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
		OrgID:           orgID,
		ToolID:          tool.ID,
		ToolName:        tool.Name,
		RequestID:       requestID,
		Actor:           actor,
		InputRedacted:   utils.Redact(input),
		ContextRedacted: utils.Redact(contextMap),
		Decision:        auditdomain.DecisionAllow,
		PolicyID:        policyID,
		Reason:          strPtr(matchReason),
		Status:          auditdomain.StatusSuccess,
		OutputRedacted:  utils.Redact(result),
		LatencyMS:       int(latency),
	})

	s.log.Info().
		Str("request_id", requestID).
		Str("org_id", orgID.String()).
		Str("tool_name", tool.Name).
		Str("decision", "allow").
		Str("status", "success").
		Int64("latency_ms", latency).
		Msg("run_success")
	return gwdomain.RunResponse{
		RequestID:  requestID,
		Decision:   gwdomain.DecisionAllow,
		ToolName:   tool.Name,
		Status:     gwdomain.RunStatusSuccess,
		Result:     result,
		LatencyMS:  latency,
		HTTPStatus: http.StatusOK,
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

func (s *service) firstMatch(policies []policydomain.Policy, input, contextMap map[string]any, tool tooldomain.Tool) (*policydomain.Policy, string, parsedLimits, error) {
	attrs := policyuc.ToolAttributes{
		Name:       tool.Name,
		Kind:       string(tool.Kind),
		Method:     tool.Method,
		URL:        tool.URL,
		ActionType: string(tool.ActionType),
		RiskLevel:  tool.RiskLevel,
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

func (s *service) blocked(ctx context.Context, orgID uuid.UUID, actor *string, requestID string, toolName string, toolIDVal uuid.UUID, policyID *uuid.UUID, reason string, code string, httpStatus int, start time.Time, input, contextMap map[string]any) gwdomain.RunResponse {
	latency := time.Since(start).Milliseconds()
	decision := auditdomain.DecisionDeny
	status := auditdomain.StatusBlocked
	rc := reason
	_ = s.auditRepo.Create(ctx, auditdomain.AuditEvent{
		OrgID:           orgID,
		ToolID:          toolIDVal,
		ToolName:        toolName,
		RequestID:       requestID,
		Actor:           actor,
		InputRedacted:   utils.Redact(input),
		ContextRedacted: utils.Redact(contextMap),
		Decision:        decision,
		PolicyID:        policyID,
		Reason:          &rc,
		Status:          status,
		OutputRedacted:  nil,
		ErrorCode:       &code,
		ErrorMessage:    &rc,
		LatencyMS:       int(latency),
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
	return gwdomain.RunResponse{
		RequestID:  requestID,
		Decision:   gwdomain.DecisionDeny,
		ToolName:   toolName,
		Status:     gwdomain.RunStatusBlocked,
		Reason:     &reason,
		ErrorCode:  &code,
		ErrorMsg:   &reason,
		LatencyMS:  latency,
		HTTPStatus: httpStatus,
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
