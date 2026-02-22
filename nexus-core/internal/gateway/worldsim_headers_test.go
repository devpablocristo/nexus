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
	"nexus-core/pkg/types"
	"nexus-core/pkg/validations/jsonschema"
)

type captureExecutor struct {
	lastHeaders map[string]string
}

func (c *captureExecutor) Execute(_ context.Context, _ string, _ string, _ map[string]any, headers map[string]string, _ int) (any, int, *types.HTTPError) {
	c.lastHeaders = map[string]string{}
	for k, v := range headers {
		c.lastHeaders[k] = v
	}
	return map[string]any{"ok": true}, 200, nil
}

func TestRun_WorldSimInternalHeaders(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	exec := &captureExecutor{}

	svc := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              toolID,
			OrgID:           orgID,
			Name:            "world.move",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "POST",
			URL:             "http://world-sim:8087/tools/world.move",
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
			EgressAllowlist:        "world-sim:8087",
			WorldSimBaseURL:        "http://world-sim:8087",
			WorldSimInternalKey:    "test-internal",
			DisableSSRFProtection:  true,
		},
		zerolog.Nop(),
	)

	reqID := "req-world-1"
	resp, err := svc.Run(context.Background(), orgID, gwdomain.RunRequest{
		RequestID: reqID,
		ToolName:  "world.move",
		Input: map[string]any{
			"org_id":     orgID.String(),
			"agent_id":   "agent-1",
			"run_id":     "run-1",
			"step_id":    1,
			"request_id": reqID,
		},
		Context:   map[string]any{},
		TimeoutMS: 5000,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Status != gwdomain.RunStatusSuccess {
		t.Fatalf("unexpected status: %s reason=%q errcode=%q errmsg=%q", resp.Status, strVal(resp.Reason), strVal(resp.ErrorCode), strVal(resp.ErrorMsg))
	}
	if got := exec.lastHeaders["X-Nexus-Request-Id"]; got != reqID {
		t.Fatalf("expected X-Nexus-Request-Id=%q got %q", reqID, got)
	}
	if got := exec.lastHeaders["X-WorldSim-Internal-Key"]; got != "test-internal" {
		t.Fatalf("expected X-WorldSim-Internal-Key set, got %q", got)
	}
}

func TestRun_NonWorldSimDoesNotGetInternalKey(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	exec := &captureExecutor{}

	svc := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              toolID,
			OrgID:           orgID,
			Name:            "transfer",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "POST",
			URL:             "https://api.example.com/transfer",
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
			EgressAllowlist:        "world-sim:8087",
			WorldSimBaseURL:        "http://world-sim:8087",
			WorldSimInternalKey:    "test-internal",
			DisableSSRFProtection:  true,
		},
		zerolog.Nop(),
	)

	reqID := "req-x-1"
	_, err := svc.Run(context.Background(), orgID, gwdomain.RunRequest{
		RequestID: reqID,
		ToolName:  "transfer",
		Input: map[string]any{
			"amount": 10,
		},
		Context:   map[string]any{},
		TimeoutMS: 5000,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := exec.lastHeaders["X-Nexus-Request-Id"]; got != reqID {
		t.Fatalf("expected X-Nexus-Request-Id=%q got %q", reqID, got)
	}
	if got := exec.lastHeaders["X-WorldSim-Internal-Key"]; got != "" {
		t.Fatalf("did not expect internal key header for non-worldsim tool, got %q", got)
	}
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
