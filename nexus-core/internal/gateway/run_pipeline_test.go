package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/internal/dlp"
	"nexus-core/internal/policy"
	policydomain "nexus-core/internal/policy/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus/pkg/types"
	"nexus/pkg/validations/jsonschema"
)

type fakeToolRepoNotFound struct{}

func (fakeToolRepoNotFound) GetByName(context.Context, uuid.UUID, string) (tooldomain.Tool, error) {
	return tooldomain.Tool{}, types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "tool not found")
}

type fakePolicyRepoWithPolicies struct {
	policies []policydomain.Policy
}

func (f fakePolicyRepoWithPolicies) ListByToolID(context.Context, uuid.UUID, uuid.UUID) ([]policydomain.Policy, error) {
	return f.policies, nil
}

type fakeLimiterDeny struct{}

func (fakeLimiterDeny) Allow(string, int) bool { return false }

type fakeEgressDeny struct{}

func (fakeEgressDeny) IsHostAllowed(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return false, nil
}

type fakeActionOverridesDeny struct{ reason string }

func (f fakeActionOverridesDeny) ResolveRuntimeOverrides(context.Context, uuid.UUID, string) (RuntimeActionOverrides, error) {
	r := f.reason
	if r == "" {
		r = "blocked by active action override"
	}
	return RuntimeActionOverrides{Deny: true, DenyReason: r}, nil
}

type fakeTenantCaps struct{ rpm int }

func (f fakeTenantCaps) GetRunRPM(context.Context, uuid.UUID) (int, error) {
	return f.rpm, nil
}

type fakeApproval struct{ id string }

func (f fakeApproval) RequestApproval(_ context.Context, _ ApprovalRequest) (string, error) {
	return f.id, nil
}

type fakeExecutorError struct {
	httpErr *types.HTTPError
}

func (f fakeExecutorError) Execute(context.Context, string, string, map[string]any, map[string]string, int) (any, int, *types.HTTPError) {
	return nil, 0, f.httpErr
}

type sleepyPolicyRepo struct {
	sleep time.Duration
}

func (s sleepyPolicyRepo) ListByToolID(context.Context, uuid.UUID, uuid.UUID) ([]policydomain.Policy, error) {
	time.Sleep(s.sleep)
	return nil, nil
}

func defaultTool() tooldomain.Tool {
	return tooldomain.Tool{
		ID:              uuid.New(),
		OrgID:           uuid.New(),
		Name:            "echo",
		Kind:            tooldomain.ToolKindHTTP,
		Method:          "GET",
		URL:             "http://mock-tools:8081/echo",
		ActionType:      tooldomain.ActionRead,
		Enabled:         true,
		InputSchemaJSON: mustJSON(map[string]any{"type": "object"}),
	}
}

func defaultConfig() Config {
	return Config{
		TimeoutBudgetDefaultMS: 10000,
		TimeoutBudgetMinMS:     1000,
		TimeoutBudgetMaxMS:     30000,
		DisableSSRFProtection:  true,
	}
}

func defaultReq() gwdomain.RunRequest {
	return gwdomain.RunRequest{
		ToolName: "echo",
		Input:    map[string]any{"hello": "world"},
		Context:  map[string]any{},
	}
}

func buildSvc(
	toolRepo ToolRepoPort,
	policyRepo PolicyRepoPort,
	limiter RateLimiterPort,
	egress EgressPort,
	executor HTTPExecutorPort,
	tenantCaps TenantLimitsPort,
	actionOverrides ActionOverridesPort,
	approval ApprovalPort,
	cfg Config,
) *Usecases {
	return NewUsecases(
		toolRepo,
		policyRepo,
		fakeAuditRepo{},
		fakeSecretRepo{},
		egress,
		limiter,
		executor,
		fakeIdempotency{},
		tenantCaps,
		actionOverrides,
		approval,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		cfg,
		zerolog.Nop(),
	)
}

func TestRunPipeline(t *testing.T) {
	orgID := uuid.New()
	tool := defaultTool()
	tool.OrgID = orgID

	t.Run("happy_path_read_allow", func(t *testing.T) {
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSuccess(t, resp)
	})

	t.Run("happy_path_write_with_idempotency", func(t *testing.T) {
		writeTool := tool
		writeTool.ActionType = tooldomain.ActionWrite
		writeTool.Method = "POST"
		allowAll := policydomain.Policy{
			ID: uuid.New(), OrgID: orgID, ToolID: writeTool.ID,
			Effect: policydomain.EffectAllow, Priority: 1, Enabled: true,
			ConditionsJSON: mustJSON(map[string]any{}),
			ReasonTemplate: "allow all",
		}
		exec := &fakeExecutor{}
		svc := buildSvc(
			fakeToolRepo{tool: writeTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{allowAll}},
			fakeLimiter{},
			fakeEgress{},
			exec,
			nil, nil, nil,
			defaultConfig(),
		)
		idk := "key-1"
		req := defaultReq()
		req.ToolName = writeTool.Name
		req.IdempotencyKey = &idk
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSuccess(t, resp)
		if exec.calls != 1 {
			t.Fatalf("expected 1 executor call, got %d", exec.calls)
		}
		if resp.Idempotency.Outcome != gwdomain.IdempotencyNew {
			t.Fatalf("expected NEW idempotency, got %s", resp.Idempotency.Outcome)
		}
	})

	t.Run("tool_not_found", func(t *testing.T) {
		svc := buildSvc(
			fakeToolRepoNotFound{},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusNotFound, types.ErrCodeNotFound)
	})

	t.Run("tool_disabled", func(t *testing.T) {
		disabled := tool
		disabled.Enabled = false
		svc := buildSvc(
			fakeToolRepo{tool: disabled},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePolicyDenied)
	})

	t.Run("policy_deny", func(t *testing.T) {
		denyPolicy := policydomain.Policy{
			ID:       uuid.New(),
			OrgID:    orgID,
			ToolID:   tool.ID,
			Effect:   policydomain.EffectDeny,
			Priority: 1,
			Enabled:  true,
			ConditionsJSON: mustJSON(map[string]any{
				"all": []any{
					map[string]any{"path": "input.amount", "op": "gt", "value": 1000},
				},
			}),
			ReasonTemplate: "amount exceeds limit",
		}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{denyPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		req := defaultReq()
		req.Input = map[string]any{"amount": 5000.0}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePolicyDenied)
		if resp.Decision != gwdomain.DecisionDeny {
			t.Fatalf("expected deny decision, got %s", resp.Decision)
		}
	})

	t.Run("policy_require_approval", func(t *testing.T) {
		approvalPolicy := policydomain.Policy{
			ID:       uuid.New(),
			OrgID:    orgID,
			ToolID:   tool.ID,
			Effect:   policydomain.EffectAllow,
			Priority: 1,
			Enabled:  true,
			ConditionsJSON: mustJSON(map[string]any{
				"all": []any{
					map[string]any{"path": "input.hello", "op": "exists"},
				},
			}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "needs human review",
		}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: "appr-123"},
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusAccepted, types.ErrCodeApprovalRequired)
	})

	t.Run("dlp_populates_context_and_policy_deny", func(t *testing.T) {
		denyOnDLP := policydomain.Policy{
			ID:       uuid.New(),
			OrgID:    orgID,
			ToolID:   tool.ID,
			Effect:   policydomain.EffectDeny,
			Priority: 1,
			Enabled:  true,
			ConditionsJSON: mustJSON(map[string]any{
				"all": []any{
					map[string]any{"path": "context.dlp.email.count", "op": "gt", "value": 0},
				},
			}),
			ReasonTemplate: "sensitive data detected",
		}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{denyOnDLP}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		req := defaultReq()
		req.Input = map[string]any{"email": "user@example.com"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePolicyDenied)
	})

	t.Run("schema_input_invalid", func(t *testing.T) {
		toolWithSchema := tool
		toolWithSchema.InputSchemaJSON = mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer"},
			},
			"required": []any{"count"},
		})
		svc := buildSvc(
			fakeToolRepo{tool: toolWithSchema},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		req := defaultReq()
		req.Input = map[string]any{"wrong_field": "abc"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusBadRequest, types.ErrCodeValidation)
	})

	t.Run("rate_limit_tenant_exceeded", func(t *testing.T) {
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiterDeny{},
			fakeEgress{},
			&fakeExecutor{},
			fakeTenantCaps{rpm: 10},
			nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodeRateLimited)
	})

	t.Run("rate_limit_tool_exceeded", func(t *testing.T) {
		policyWithRL := policydomain.Policy{
			ID:       uuid.New(),
			OrgID:    orgID,
			ToolID:   tool.ID,
			Effect:   policydomain.EffectAllow,
			Priority: 1,
			Enabled:  true,
			ConditionsJSON: mustJSON(map[string]any{
				"all": []any{
					map[string]any{"path": "input.hello", "op": "exists"},
				},
			}),
			LimitsJSON:     mustJSON(map[string]any{"rate_limit": map[string]any{"per_minute": 5}}),
			ReasonTemplate: "allowed with rate limit",
		}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{policyWithRL}},
			fakeLimiterDeny{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodeRateLimited)
	})

	t.Run("egress_host_denied", func(t *testing.T) {
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgressDeny{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodeEgressDenied)
	})

	t.Run("timeout_budget_exhausted", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.TimeoutBudgetDefaultMS = 1
		cfg.TimeoutBudgetMinMS = 1
		cfg.TimeoutBudgetMaxMS = 1

		svc := buildSvc(
			fakeToolRepo{tool: tool},
			sleepyPolicyRepo{sleep: 5 * time.Millisecond},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			cfg,
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusRequestTimeout, types.ErrCodeTimeoutBudget)
	})

	t.Run("upstream_5xx", func(t *testing.T) {
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			fakeExecutorError{httpErr: &types.HTTPError{
				Status:  http.StatusBadGateway,
				Code:    types.ErrCodeUpstream5xx,
				Message: "upstream returned 500",
			}},
			nil, nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusBadGateway, types.ErrCodeUpstream5xx)
		if resp.Status != gwdomain.RunStatusError {
			t.Fatalf("expected error status, got %s", resp.Status)
		}
	})

	t.Run("output_schema_invalid", func(t *testing.T) {
		toolWithOutSchema := tool
		toolWithOutSchema.OutputSchemaJSON = mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"result_id": map[string]any{"type": "integer"},
			},
			"required": []any{"result_id"},
		})
		svc := buildSvc(
			fakeToolRepo{tool: toolWithOutSchema},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusBadGateway, types.ErrCodeOutputSchemaInvalid)
	})

	t.Run("action_override_deny", func(t *testing.T) {
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil,
			fakeActionOverridesDeny{reason: "emergency lockdown"},
			nil,
			defaultConfig(),
		)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePolicyDenied)
	})

	t.Run("write_tool_default_deny_without_policy", func(t *testing.T) {
		writeTool := tool
		writeTool.ActionType = tooldomain.ActionWrite
		writeTool.Method = "POST"
		svc := buildSvc(
			fakeToolRepo{tool: writeTool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = writeTool.Name
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePolicyDenied)
		if resp.Decision != gwdomain.DecisionDeny {
			t.Fatalf("expected deny, got %s", resp.Decision)
		}
	})

	t.Run("policy_allow_bypasses_default_deny_for_write", func(t *testing.T) {
		writeTool := tool
		writeTool.ActionType = tooldomain.ActionWrite
		writeTool.Method = "POST"
		allowPolicy := policydomain.Policy{
			ID:       uuid.New(),
			OrgID:    orgID,
			ToolID:   writeTool.ID,
			Effect:   policydomain.EffectAllow,
			Priority: 1,
			Enabled:  true,
			ConditionsJSON: mustJSON(map[string]any{
				"all": []any{
					map[string]any{"path": "input.hello", "op": "exists"},
				},
			}),
			ReasonTemplate: "allowed",
		}
		svc := buildSvc(
			fakeToolRepo{tool: writeTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{allowPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = writeTool.Name
		idk := "key-w"
		req.IdempotencyKey = &idk
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSuccess(t, resp)
	})
}

func assertSuccess(t *testing.T, resp gwdomain.RunResponse) {
	t.Helper()
	if resp.Status != gwdomain.RunStatusSuccess {
		t.Fatalf("expected success, got %s (http=%d code=%s reason=%s)", resp.Status, resp.HTTPStatus, ptrVal(resp.ErrorCode), ptrVal(resp.Reason))
	}
	if resp.Decision != gwdomain.DecisionAllow {
		t.Fatalf("expected allow, got %s", resp.Decision)
	}
	if resp.HTTPStatus != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.HTTPStatus)
	}
}

func assertBlocked(t *testing.T, resp gwdomain.RunResponse, expectedHTTP int, expectedCode string) {
	t.Helper()
	if resp.HTTPStatus != expectedHTTP {
		t.Fatalf("expected %d, got %d (status=%s code=%s reason=%s)", expectedHTTP, resp.HTTPStatus, resp.Status, ptrVal(resp.ErrorCode), ptrVal(resp.Reason))
	}
	if resp.ErrorCode == nil || *resp.ErrorCode != expectedCode {
		t.Fatalf("expected %s, got %s (reason=%s)", expectedCode, ptrVal(resp.ErrorCode), ptrVal(resp.Reason))
	}
}

func ptrVal(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
