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
	"nexus/pkg/types"
	"nexus/pkg/validations/jsonschema"
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

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestRun_SSRFAllowlist_AllowsAllowlistedHostPort(t *testing.T) {
	orgID := uuid.New()
	exec := &captureExecutor{}
	service := NewUsecases(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              uuid.New(),
			OrgID:           orgID,
			Name:            "echo",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "POST",
			URL:             "http://mock-tools:8081/tools/echo",
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
		nil,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{
			TimeoutBudgetDefaultMS: 10000,
			TimeoutBudgetMinMS:     1000,
			TimeoutBudgetMaxMS:     30000,
			EgressAllowlist:        "mock-tools:8081",
			DisableSSRFProtection:  false,
		},
		zerolog.Nop(),
	)

	resp, err := service.Run(context.Background(), orgID, gwdomain.RunRequest{
		RequestID: "req-1",
		ToolName:  "echo",
		Input:     map[string]any{"hello": "e2e"},
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
	service := NewUsecases(
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
		nil,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{
			TimeoutBudgetDefaultMS: 10000,
			TimeoutBudgetMinMS:     1000,
			TimeoutBudgetMaxMS:     30000,
			EgressAllowlist:        "mock-tools:8081",
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
