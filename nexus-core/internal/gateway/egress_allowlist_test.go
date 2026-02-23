package gateway

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-core/internal/dlp"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/internal/policy"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus-core/pkg/validations/jsonschema"
)

func TestRun_SSRFAllowlist_AllowsSimEngineHostPort(t *testing.T) {
	orgID := uuid.New()
	exec := &captureExecutor{}
	service := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              uuid.New(),
			OrgID:           orgID,
			Name:            "world.observe",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "POST",
			URL:             "http://sim-engine:8087/tools/world.observe",
			InputSchemaJSON: []byte(`{"type":"object"}`),
			ActionType:      tooldomain.ActionRead,
			Enabled:         true,
		}},
		fakePolicyRepo{},
		fakeAuditRepo{},
		fakeSecretRepo{},
		fakeEgress{},
		fakeLimiter{},
		exec,
		fakeIdempotency{},
		nil,
		nil,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{
			TimeoutBudgetDefaultMS: 10000,
			TimeoutBudgetMinMS:     1000,
			TimeoutBudgetMaxMS:     30000,
			EgressAllowlist:        "sim-engine:8087",
			DisableSSRFProtection:  false,
		},
		zerolog.Nop(),
	)

	resp, err := service.Run(context.Background(), orgID, gwdomain.RunRequest{
		RequestID: "req-1",
		ToolName:  "world.observe",
		Input:     map[string]any{"org_id": orgID.String(), "run_id": "r1", "agent_id": "a1", "step_id": 0},
		Context:   map[string]any{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Status != gwdomain.RunStatusSuccess {
		t.Fatalf("expected success got status=%s reason=%q", resp.Status, strVal(resp.Reason))
	}
}

func TestRun_SSRFAllowlist_BlocksPrivateHostNotAllowlisted(t *testing.T) {
	orgID := uuid.New()
	exec := &captureExecutor{}
	service := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              uuid.New(),
			OrgID:           orgID,
			Name:            "internal.call",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "POST",
			URL:             "http://10.1.2.3:8087/tools/x",
			InputSchemaJSON: []byte(`{"type":"object"}`),
			ActionType:      tooldomain.ActionRead,
			Enabled:         true,
		}},
		fakePolicyRepo{},
		fakeAuditRepo{},
		fakeSecretRepo{},
		fakeEgress{},
		fakeLimiter{},
		exec,
		fakeIdempotency{},
		nil,
		nil,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{
			TimeoutBudgetDefaultMS: 10000,
			TimeoutBudgetMinMS:     1000,
			TimeoutBudgetMaxMS:     30000,
			EgressAllowlist:        "sim-engine:8087",
			DisableSSRFProtection:  false,
		},
		zerolog.Nop(),
	)

	resp, err := service.Run(context.Background(), orgID, gwdomain.RunRequest{
		RequestID: "req-2",
		ToolName:  "internal.call",
		Input:     map[string]any{},
		Context:   map[string]any{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Status != gwdomain.RunStatusBlocked {
		t.Fatalf("expected blocked got status=%s", resp.Status)
	}
	if strVal(resp.ErrorCode) != "EGRESS_DENIED" {
		t.Fatalf("expected EGRESS_DENIED got %q", strVal(resp.ErrorCode))
	}
}
