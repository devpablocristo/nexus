package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-core/internal/dlp"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
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
func (fakeToolRepoNotFound) GetByID(context.Context, uuid.UUID, uuid.UUID) (tooldomain.Tool, error) {
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

type fakeProtectedResources struct {
	items []ProtectedResource
}

func (f fakeProtectedResources) ListProtectedResources(context.Context, uuid.UUID) ([]ProtectedResource, error) {
	return append([]ProtectedResource{}, f.items...), nil
}

type fakeRestoreEvidence struct {
	items []RestoreEvidence
}

func (f fakeRestoreEvidence) ListRestoreEvidence(context.Context, uuid.UUID, string, int) ([]RestoreEvidence, error) {
	return append([]RestoreEvidence{}, f.items...), nil
}

type fakeIntentRepo struct {
	created     gwdomain.ExecutionIntent
	getByID     gwdomain.ExecutionIntent
	listItems   []gwdomain.ExecutionIntent
	linked      bool
	linkedID    uuid.UUID
	approvalID  uuid.UUID
	executed    uuid.UUID
	createCount int
}

func (f *fakeIntentRepo) Create(_ context.Context, intent gwdomain.ExecutionIntent) (gwdomain.ExecutionIntent, error) {
	f.createCount++
	intent.ID = uuid.New()
	f.created = intent
	if f.getByID.ID == uuid.Nil {
		f.getByID = intent
	}
	return intent, nil
}

func (f *fakeIntentRepo) GetByID(_ context.Context, _ uuid.UUID, id uuid.UUID) (gwdomain.ExecutionIntent, error) {
	if f.getByID.ID == uuid.Nil {
		f.getByID.ID = id
	}
	return f.getByID, nil
}

func (f *fakeIntentRepo) ListRecent(_ context.Context, _ uuid.UUID, _ int) ([]gwdomain.ExecutionIntent, error) {
	return append([]gwdomain.ExecutionIntent{}, f.listItems...), nil
}

func (f *fakeIntentRepo) LinkApproval(_ context.Context, _ uuid.UUID, intentID, approvalID uuid.UUID) error {
	f.linked = true
	f.linkedID = intentID
	f.approvalID = approvalID
	return nil
}

func (f *fakeIntentRepo) MarkExecuted(_ context.Context, _ uuid.UUID, intentID uuid.UUID) error {
	f.executed = intentID
	return nil
}

type fakeLeaseRepo struct {
	created gwdomain.ExecutionLease
	getByID gwdomain.ExecutionLease
	used    uuid.UUID
	expired uuid.UUID
	revoked uuid.UUID
}

func (f *fakeLeaseRepo) Create(_ context.Context, lease gwdomain.ExecutionLease) (gwdomain.ExecutionLease, error) {
	lease.ID = uuid.New()
	lease.CreatedAt = time.Now().UTC()
	f.created = lease
	if f.getByID.ID == uuid.Nil {
		f.getByID = lease
	}
	return lease, nil
}

func (f *fakeLeaseRepo) GetByID(_ context.Context, _ uuid.UUID, leaseID uuid.UUID) (gwdomain.ExecutionLease, error) {
	if f.getByID.ID == uuid.Nil {
		f.getByID.ID = leaseID
	}
	return f.getByID, nil
}

func (f *fakeLeaseRepo) MarkUsed(_ context.Context, _ uuid.UUID, leaseID uuid.UUID) error {
	f.used = leaseID
	return nil
}

func (f *fakeLeaseRepo) MarkExpired(_ context.Context, _ uuid.UUID, leaseID uuid.UUID) error {
	f.expired = leaseID
	return nil
}

func (f *fakeLeaseRepo) MarkRevoked(_ context.Context, _ uuid.UUID, leaseID uuid.UUID) error {
	f.revoked = leaseID
	return nil
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

func terraformTool(orgID uuid.UUID) tooldomain.Tool {
	return tooldomain.Tool{
		ID:              uuid.New(),
		OrgID:           orgID,
		Name:            "terraform-aws-apply",
		Kind:            tooldomain.ToolKindHTTP,
		Method:          "POST",
		URL:             "http://mock-tools:8081/terraform/apply",
		ActionType:      tooldomain.ActionWrite,
		Classification:  "external",
		Sensitivity:     "high",
		RiskLevel:       5,
		Enabled:         true,
		InputSchemaJSON: mustJSON(map[string]any{"type": "object"}),
	}
}

func kubectlTool(orgID uuid.UUID) tooldomain.Tool {
	return tooldomain.Tool{
		ID:              uuid.New(),
		OrgID:           orgID,
		Name:            "kubectl-apply",
		Kind:            tooldomain.ToolKindHTTP,
		Method:          "POST",
		URL:             "http://mock-tools:8081/kubectl/apply",
		ActionType:      tooldomain.ActionWrite,
		Classification:  "external",
		Sensitivity:     "high",
		RiskLevel:       4,
		Enabled:         true,
		InputSchemaJSON: mustJSON(map[string]any{"type": "object"}),
	}
}

func bashTool(orgID uuid.UUID) tooldomain.Tool {
	return tooldomain.Tool{
		ID:              uuid.New(),
		OrgID:           orgID,
		Name:            "bash-remote",
		Kind:            tooldomain.ToolKindHTTP,
		Method:          "POST",
		URL:             "http://mock-tools:8081/bash/run",
		ActionType:      tooldomain.ActionWrite,
		Classification:  "external",
		Sensitivity:     "high",
		RiskLevel:       4,
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
		LeaseTokenIssuer:       "nexus-core-test",
		LeaseTokenSigningKey:   "test-lease-signing-key",
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
		approvalID := uuid.NewString()
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
		intentRepo := &fakeIntentRepo{}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: approvalID},
			defaultConfig(),
		)
		svc.WithIntentRepo(intentRepo)
		resp, err := svc.Run(context.Background(), orgID, defaultReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusAccepted, types.ErrCodeApprovalRequired)
		if resp.IntentID == nil || *resp.IntentID == "" {
			t.Fatal("expected intent_id in blocked response")
		}
		if resp.ApprovalID == nil || *resp.ApprovalID != approvalID {
			t.Fatalf("expected approval id %s, got %s", approvalID, ptrVal(resp.ApprovalID))
		}
		if resp.RiskClass == nil || *resp.RiskClass == "" {
			t.Fatal("expected risk class in blocked response")
		}
		if intentRepo.createCount != 1 {
			t.Fatalf("expected one intent creation, got %d", intentRepo.createCount)
		}
		if !intentRepo.linked {
			t.Fatal("expected approval to link with intent")
		}
	})

	t.Run("terraform_preflight_requires_plan_artifact", func(t *testing.T) {
		tfTool := terraformTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         tfTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "terraform approval required",
		}
		intentRepo := &fakeIntentRepo{}
		svc := buildSvc(
			fakeToolRepo{tool: tfTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		svc.WithIntentRepo(intentRepo)
		req := defaultReq()
		req.ToolName = tfTool.Name
		req.Context = map[string]any{"target_env": "production", "workspace": "prod"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusBadRequest, types.ErrCodePreflightFailed)
		if intentRepo.createCount != 0 {
			t.Fatalf("expected no intent creation, got %d", intentRepo.createCount)
		}
	})

	t.Run("terraform_preflight_persists_artifact_hash", func(t *testing.T) {
		tfTool := terraformTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         tfTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "terraform approval required",
		}
		intentRepo := &fakeIntentRepo{}
		completedAt := time.Now().UTC()
		svc := buildSvc(
			fakeToolRepo{tool: tfTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		).WithRestoreEvidence(fakeRestoreEvidence{
			items: []RestoreEvidence{{
				ID:          uuid.New(),
				Environment: "prod",
				System:      "database",
				Status:      "passed",
				SnapshotID:  "snap-123",
				CompletedAt: &completedAt,
				Source:      "dr.test_restore.sh",
			}},
		})
		svc.WithIntentRepo(intentRepo)
		req := defaultReq()
		req.ToolName = tfTool.Name
		req.Input = map[string]any{
			"terraform_plan_json": map[string]any{"format_version": "1.2", "resource_changes": []any{}},
			"backend":             "s3",
			"dynamodb_table":      "tf-locks",
			"plan_summary":        map[string]any{"create": 1, "update": 0, "delete": 0, "replace": 0},
		}
		req.Context = map[string]any{"target_env": "production", "workspace": "prod"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusAccepted, types.ErrCodeApprovalRequired)
		if intentRepo.created.PreflightStatus != gwdomain.PreflightStatusPassed {
			t.Fatalf("expected passed preflight, got %s", intentRepo.created.PreflightStatus)
		}
		if intentRepo.created.PreflightArtifactSHA == nil || *intentRepo.created.PreflightArtifactSHA == "" {
			t.Fatal("expected preflight artifact sha")
		}
	})

	t.Run("terraform_preflight_blocks_destructive_prod", func(t *testing.T) {
		tfTool := terraformTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         tfTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "terraform approval required",
		}
		svc := buildSvc(
			fakeToolRepo{tool: tfTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = tfTool.Name
		req.Input = map[string]any{
			"terraform_plan_json": "plan-contents",
			"backend":             "s3",
			"dynamodb_table":      "tf-locks",
			"plan_summary":        map[string]any{"create": 0, "update": 1, "delete": 1, "replace": 0},
		}
		req.Context = map[string]any{"target_env": "production", "workspace": "prod"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePreflightFailed)
	})

	t.Run("terraform_preflight_requires_recent_restore_evidence", func(t *testing.T) {
		tfTool := terraformTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         tfTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "terraform approval required",
		}
		svc := buildSvc(
			fakeToolRepo{tool: tfTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = tfTool.Name
		req.Input = map[string]any{
			"terraform_plan_json": "plan-contents",
			"backend":             "s3",
			"dynamodb_table":      "tf-locks",
			"plan_summary":        map[string]any{"create": 0, "update": 1, "delete": 0, "replace": 0},
		}
		req.Context = map[string]any{"target_env": "production", "workspace": "prod"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusBadRequest, types.ErrCodePreflightFailed)
		if resp.Reason == nil || !strings.Contains(*resp.Reason, "restore evidence") {
			t.Fatalf("expected restore evidence reason, got %#v", resp.Reason)
		}
	})

	t.Run("terraform_preflight_blocks_protected_resource", func(t *testing.T) {
		tfTool := terraformTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         tfTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "terraform approval required",
		}
		svc := buildSvc(
			fakeToolRepo{tool: tfTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		).WithProtectedResources(fakeProtectedResources{
			items: []ProtectedResource{
				{
					ID:           uuid.New(),
					Name:         "prod-state-bucket",
					ResourceType: "terraform_address",
					MatchValue:   "aws_s3_bucket.prod_state",
					MatchMode:    "exact",
					Environment:  "prod",
					Reason:       "state backend crown jewel",
				},
			},
		})
		req := defaultReq()
		req.ToolName = tfTool.Name
		req.Input = map[string]any{
			"terraform_plan_json": map[string]any{"resource_changes": []any{"aws_s3_bucket.prod_state"}},
			"backend":             "s3",
			"dynamodb_table":      "tf-locks",
			"plan_summary":        map[string]any{"create": 0, "update": 1, "delete": 0, "replace": 0},
			"resource_refs":       []any{"aws_s3_bucket.prod_state"},
		}
		req.Context = map[string]any{"target_env": "production", "workspace": "prod"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePreflightFailed)
		if resp.Reason == nil || *resp.Reason == "" {
			t.Fatal("expected protected resource block reason")
		}
	})

	t.Run("kubectl_preflight_requires_cluster_and_namespace", func(t *testing.T) {
		kTool := kubectlTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         kTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "kubectl approval required",
		}
		svc := buildSvc(
			fakeToolRepo{tool: kTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = kTool.Name
		req.Input = map[string]any{"command": "kubectl apply -f deployment.yaml"}
		req.Context = map[string]any{"target_env": "production"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusBadRequest, types.ErrCodePreflightFailed)
	})

	t.Run("kubectl_preflight_blocks_delete_in_prod", func(t *testing.T) {
		kTool := kubectlTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         kTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "kubectl approval required",
		}
		svc := buildSvc(
			fakeToolRepo{tool: kTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = kTool.Name
		req.Input = map[string]any{
			"command":   "kubectl delete deployment api",
			"cluster":   "prod-cluster",
			"namespace": "default",
			"verb":      "delete",
		}
		req.Context = map[string]any{"target_env": "production"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePreflightFailed)
	})

	t.Run("bash_preflight_requires_target_host", func(t *testing.T) {
		bTool := bashTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         bTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "bash approval required",
		}
		svc := buildSvc(
			fakeToolRepo{tool: bTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = bTool.Name
		req.Input = map[string]any{"command": "echo hi"}
		req.Context = map[string]any{"target_env": "production"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusBadRequest, types.ErrCodePreflightFailed)
	})

	t.Run("bash_preflight_blocks_destructive_prod", func(t *testing.T) {
		bTool := bashTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         bTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "bash approval required",
		}
		svc := buildSvc(
			fakeToolRepo{tool: bTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		req := defaultReq()
		req.ToolName = bTool.Name
		req.Input = map[string]any{
			"command":     "rm -rf /var/lib/app",
			"target_host": "prod-host-1",
		}
		req.Context = map[string]any{"target_env": "production"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePreflightFailed)
	})

	t.Run("bash_preflight_blocks_protected_host", func(t *testing.T) {
		bTool := bashTool(orgID)
		approvalPolicy := policydomain.Policy{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         bTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "bash approval required",
		}
		intentRepo := &fakeIntentRepo{}
		svc := buildSvc(
			fakeToolRepo{tool: bTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		).WithProtectedResources(fakeProtectedResources{
			items: []ProtectedResource{
				{
					ID:           uuid.New(),
					Name:         "prod-primary-db",
					ResourceType: "host",
					MatchValue:   "db-prod.internal",
					MatchMode:    "contains",
					Environment:  "prod",
					Reason:       "primary database",
				},
			},
		})
		svc.WithIntentRepo(intentRepo)
		req := defaultReq()
		req.ToolName = bTool.Name
		req.Input = map[string]any{
			"command":     "pg_dump -h db-prod.internal appdb",
			"target_host": "db-prod.internal",
		}
		req.Context = map[string]any{"target_env": "production"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodePreflightFailed)
		if intentRepo.createCount != 0 {
			t.Fatalf("expected no intent creation, got %d", intentRepo.createCount)
		}
	})

	t.Run("execute_intent_runs_approved_request", func(t *testing.T) {
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
			ReasonTemplate: "approved intent execution",
		}
		intentID := uuid.New()
		approvalID := uuid.New()
		leaseID := uuid.New()
		intentRepo := &fakeIntentRepo{
			getByID: gwdomain.ExecutionIntent{
				ID:         intentID,
				OrgID:      orgID,
				ToolID:     writeTool.ID,
				ToolName:   writeTool.Name,
				RequestID:  "intent-r1",
				Scopes:     []string{"gateway:run"},
				Input:      map[string]any{"hello": "world"},
				Context:    map[string]any{},
				RiskClass:  gwdomain.RiskClassMutateProd,
				Reason:     "approved",
				ApprovalID: &approvalID,
				Status:     gwdomain.IntentStatusApproved,
				ExpiresAt:  time.Now().Add(time.Hour),
			},
		}
		leaseRepo := &fakeLeaseRepo{
			getByID: gwdomain.ExecutionLease{
				ID:             leaseID,
				IntentID:       intentID,
				ToolName:       writeTool.Name,
				RiskClass:      gwdomain.RiskClassMutateProd,
				Status:         gwdomain.ExecutionLeaseStatusActive,
				CredentialMode: "aws_sts",
				CredentialHints: map[string]any{
					"provider": "aws",
				},
				ExpiresAt: time.Now().Add(5 * time.Minute),
			},
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
		svc.WithIntentRepo(intentRepo)
		svc.WithLeaseRepo(leaseRepo)
		resp, err := svc.ExecuteIntentWithLease(context.Background(), orgID, intentID, leaseID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertSuccess(t, resp)
		if resp.IntentID == nil || *resp.IntentID != intentID.String() {
			t.Fatalf("expected intent id %s, got %s", intentID, ptrVal(resp.IntentID))
		}
		if intentRepo.executed != intentID {
			t.Fatalf("expected executed intent %s, got %s", intentID, intentRepo.executed)
		}
		if resp.LeaseID == nil || *resp.LeaseID != leaseID.String() {
			t.Fatalf("expected lease id %s, got %s", leaseID, ptrVal(resp.LeaseID))
		}
		if leaseRepo.used != leaseID {
			t.Fatalf("expected used lease %s, got %s", leaseID, leaseRepo.used)
		}
	})

	t.Run("execute_intent_marks_expired_lease", func(t *testing.T) {
		intentID := uuid.New()
		approvalID := uuid.New()
		leaseID := uuid.New()
		intentRepo := &fakeIntentRepo{
			getByID: gwdomain.ExecutionIntent{
				ID:         intentID,
				OrgID:      orgID,
				ToolID:     tool.ID,
				ToolName:   tool.Name,
				RequestID:  "intent-expired-lease",
				Scopes:     []string{"gateway:run"},
				Input:      map[string]any{"hello": "world"},
				Context:    map[string]any{},
				RiskClass:  gwdomain.RiskClassMutateProd,
				Reason:     "approved",
				ApprovalID: &approvalID,
				Status:     gwdomain.IntentStatusApproved,
				ExpiresAt:  time.Now().Add(time.Hour),
			},
		}
		leaseRepo := &fakeLeaseRepo{
			getByID: gwdomain.ExecutionLease{
				ID:             leaseID,
				IntentID:       intentID,
				ToolName:       tool.Name,
				RiskClass:      gwdomain.RiskClassMutateProd,
				Status:         gwdomain.ExecutionLeaseStatusActive,
				CredentialMode: "lease_only",
				ExpiresAt:      time.Now().Add(-time.Minute),
			},
		}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		svc.WithIntentRepo(intentRepo)
		svc.WithLeaseRepo(leaseRepo)
		resp, err := svc.ExecuteIntentWithLease(context.Background(), orgID, intentID, leaseID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusForbidden, types.ErrCodeLeaseExpired)
		if leaseRepo.expired != leaseID {
			t.Fatalf("expected expired lease %s, got %s", leaseID, leaseRepo.expired)
		}
	})

	t.Run("issue_execution_lease_for_approved_intent", func(t *testing.T) {
		intentID := uuid.New()
		intentRepo := &fakeIntentRepo{
			getByID: gwdomain.ExecutionIntent{
				ID:         intentID,
				OrgID:      orgID,
				ToolID:     tool.ID,
				ToolName:   "terraform-aws-apply",
				RequestID:  "intent-r2",
				Input:      map[string]any{"environment": "prod"},
				Context:    map[string]any{},
				RiskClass:  gwdomain.RiskClassMutateProd,
				Status:     gwdomain.IntentStatusApproved,
				ExpiresAt:  time.Now().Add(time.Hour),
				ApprovalID: uuidPtr(uuid.New()),
			},
		}
		leaseRepo := &fakeLeaseRepo{}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		svc.WithIntentRepo(intentRepo)
		svc.WithLeaseRepo(leaseRepo)
		lease, err := svc.IssueExecutionLease(context.Background(), orgID, intentID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lease.IntentID != intentID {
			t.Fatalf("expected intent id %s, got %s", intentID, lease.IntentID)
		}
		if lease.CredentialMode != "aws_sts" {
			t.Fatalf("expected aws_sts credential mode, got %s", lease.CredentialMode)
		}
		if leaseRepo.created.ID == uuid.Nil {
			t.Fatalf("expected lease to be persisted")
		}
	})

	t.Run("risk_class_uses_prod_signals", func(t *testing.T) {
		writeTool := tool
		writeTool.ActionType = tooldomain.ActionWrite
		writeTool.Method = "POST"
		writeTool.RiskLevel = 5
		writeTool.Sensitivity = "high"
		approvalPolicy := policydomain.Policy{
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
			LimitsJSON:     mustJSON(map[string]any{"require_approval": true}),
			ReasonTemplate: "prod mutation",
		}
		intentRepo := &fakeIntentRepo{}
		svc := buildSvc(
			fakeToolRepo{tool: writeTool},
			fakePolicyRepoWithPolicies{policies: []policydomain.Policy{approvalPolicy}},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil,
			fakeApproval{id: uuid.NewString()},
			defaultConfig(),
		)
		svc.WithIntentRepo(intentRepo)
		req := defaultReq()
		req.ToolName = writeTool.Name
		req.Context = map[string]any{"target_env": "production"}
		resp, err := svc.Run(context.Background(), orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBlocked(t, resp, http.StatusAccepted, types.ErrCodeApprovalRequired)
		if resp.RiskClass == nil || *resp.RiskClass != string(gwdomain.RiskClassMutateProd) {
			t.Fatalf("expected risk class %s, got %s", gwdomain.RiskClassMutateProd, ptrVal(resp.RiskClass))
		}
	})

	t.Run("list_intents_returns_repo_items", func(t *testing.T) {
		intentID := uuid.New()
		intentRepo := &fakeIntentRepo{
			listItems: []gwdomain.ExecutionIntent{{
				ID:        intentID,
				OrgID:     orgID,
				ToolID:    tool.ID,
				ToolName:  tool.Name,
				RequestID: "req-1",
				RiskClass: gwdomain.RiskClassRead,
				Status:    gwdomain.IntentStatusApproved,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
			}},
		}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		svc.WithIntentRepo(intentRepo)
		items, err := svc.ListIntents(context.Background(), orgID, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].ID != intentID {
			t.Fatalf("expected intent %s, got %#v", intentID, items)
		}
	})

	t.Run("get_intent_preflight_returns_review", func(t *testing.T) {
		intentID := uuid.New()
		approvalID := uuid.New()
		completedAt := time.Now().UTC()
		intentRepo := &fakeIntentRepo{
			getByID: gwdomain.ExecutionIntent{
				ID:                   intentID,
				OrgID:                orgID,
				ToolID:               tool.ID,
				ToolName:             tool.Name,
				RiskClass:            gwdomain.RiskClassMutateProd,
				Reason:               "needs review",
				ApprovalID:           &approvalID,
				Status:               gwdomain.IntentStatusPendingApproval,
				PreflightStatus:      gwdomain.PreflightStatusPassed,
				PreflightSummary:     map[string]any{"family": "terraform_aws"},
				PreflightArtifactSHA: strPtr("abc123"),
				PreflightCompletedAt: &completedAt,
			},
		}
		svc := buildSvc(
			fakeToolRepo{tool: tool},
			fakePolicyRepo{},
			fakeLimiter{},
			fakeEgress{},
			&fakeExecutor{},
			nil, nil, nil,
			defaultConfig(),
		)
		svc.WithIntentRepo(intentRepo)
		review, err := svc.GetIntentPreflight(context.Background(), orgID, intentID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if review.IntentID != intentID {
			t.Fatalf("expected intent %s got %s", intentID, review.IntentID)
		}
		if review.Status != gwdomain.PreflightStatusPassed {
			t.Fatalf("expected passed preflight got %s", review.Status)
		}
		if review.ArtifactSHA256 == nil || *review.ArtifactSHA256 != "abc123" {
			t.Fatalf("expected artifact sha abc123 got %s", ptrVal(review.ArtifactSHA256))
		}
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

func TestSimulateTerraformPreflight(t *testing.T) {
	orgID := uuid.New()
	tfTool := terraformTool(orgID)
	svc := buildSvc(
		fakeToolRepo{tool: tfTool},
		fakePolicyRepoWithPolicies{policies: []policydomain.Policy{{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         tfTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			ReasonTemplate: "allowed",
		}}},
		fakeLimiter{},
		fakeEgress{},
		&fakeExecutor{},
		nil, nil, nil,
		defaultConfig(),
	)
	resp, err := svc.Simulate(context.Background(), orgID, gwdomain.RunRequest{
		ToolName: tfTool.Name,
		Input:    map[string]any{"backend": "s3"},
		Context:  map[string]any{"target_env": "production", "workspace": "prod"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Decision != gwdomain.DecisionDeny {
		t.Fatalf("expected deny, got %s", resp.Decision)
	}
	if got, _ := resp.Explain["preflight_status"].(string); got != string(gwdomain.PreflightStatusFailed) {
		t.Fatalf("expected failed preflight, got %#v", resp.Explain["preflight_status"])
	}
	if _, ok := resp.Explain["preflight_summary"]; !ok {
		t.Fatal("expected preflight summary in explain")
	}
}

func TestSimulateKubectlPreflight(t *testing.T) {
	orgID := uuid.New()
	kTool := kubectlTool(orgID)
	svc := buildSvc(
		fakeToolRepo{tool: kTool},
		fakePolicyRepoWithPolicies{policies: []policydomain.Policy{{
			ID:             uuid.New(),
			OrgID:          orgID,
			ToolID:         kTool.ID,
			Effect:         policydomain.EffectAllow,
			Priority:       1,
			Enabled:        true,
			ConditionsJSON: mustJSON(map[string]any{}),
			ReasonTemplate: "allowed",
		}}},
		fakeLimiter{},
		fakeEgress{},
		&fakeExecutor{},
		nil, nil, nil,
		defaultConfig(),
	)
	resp, err := svc.Simulate(context.Background(), orgID, gwdomain.RunRequest{
		ToolName: kTool.Name,
		Input: map[string]any{
			"command": "kubectl delete pod api-0",
			"cluster": "prod-cluster",
		},
		Context: map[string]any{"target_env": "production"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Decision != gwdomain.DecisionDeny {
		t.Fatalf("expected deny, got %s", resp.Decision)
	}
	if got, _ := resp.Explain["preflight_status"].(string); got != string(gwdomain.PreflightStatusFailed) {
		t.Fatalf("expected failed preflight, got %#v", resp.Explain["preflight_status"])
	}
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

func uuidPtr(v uuid.UUID) *uuid.UUID {
	return &v
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
