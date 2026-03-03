package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-core/internal/dlp"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/internal/policy"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus-core/pkg/validations/jsonschema"
)

type alwaysDenyLimiter struct{}

func (alwaysDenyLimiter) Allow(string, int) bool { return false }

func TestRun_WorldPolicyDenied_EmitsEnforcementEvent(t *testing.T) {
	orgID := uuid.New()
	var got map[string]any
	var gotHeaderReqID string
	var gotHeaderKey string

	simEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/run/enforcement" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotHeaderReqID = r.Header.Get("X-Nexus-Request-Id")
		gotHeaderKey = r.Header.Get("X-Sim-Engine-Internal-Key")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer simEngine.Close()

	exec := &fakeExecutor{}
	svc := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              uuid.New(),
			OrgID:           orgID,
			Name:            "world.move",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "POST",
			URL:             "http://sim-engine:8087/tools/world.move",
			InputSchemaJSON: []byte(`{"type":"object"}`),
			ActionType:      tooldomain.ActionWrite,
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
			DefaultRateLimitPerMinute: 60,
			TimeoutBudgetDefaultMS:    10000,
			TimeoutBudgetMinMS:        1000,
			TimeoutBudgetMaxMS:        30000,
			DisableSSRFProtection:     true,
			SimEngineBaseURL:          simEngine.URL,
			SimEngineInternalKey:      "sim-key",
		},
		zerolog.Nop(),
	)

	reqID := "req-world-deny-1"
	resp, err := svc.Run(context.Background(), orgID, gwdomain.RunRequest{
		RequestID: reqID,
		ToolName:  "world.move",
		Input: map[string]any{
			"org_id":   orgID.String(),
			"run_id":   "run-1",
			"agent_id": "agent-001",
			"step_id":  4,
		},
		Context: map[string]any{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Status != gwdomain.RunStatusBlocked {
		t.Fatalf("expected blocked got %s", resp.Status)
	}
	if exec.calls != 0 {
		t.Fatalf("expected no upstream execution when policy denied, got %d", exec.calls)
	}
	if got == nil {
		t.Fatalf("expected enforcement payload")
	}
	if gotHeaderReqID != reqID {
		t.Fatalf("expected request id header %q got %q", reqID, gotHeaderReqID)
	}
	if gotHeaderKey != "sim-key" {
		t.Fatalf("expected sim-engine key header, got %q", gotHeaderKey)
	}
	if got["event_type"] != "tool.denied" {
		t.Fatalf("expected tool.denied got %#v", got["event_type"])
	}
	if got["run_id"] != "run-1" {
		t.Fatalf("expected run_id run-1 got %#v", got["run_id"])
	}
}

func TestRun_WorldRateLimited_EmitsEnforcementEvent(t *testing.T) {
	orgID := uuid.New()
	var got map[string]any

	simEngine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/run/enforcement" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer simEngine.Close()

	svc := NewService(
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
		alwaysDenyLimiter{},
		&fakeExecutor{},
		fakeIdempotency{},
		nil,
		nil,
		nil,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{
			DefaultRateLimitPerMinute: 60,
			TimeoutBudgetDefaultMS:    10000,
			TimeoutBudgetMinMS:        1000,
			TimeoutBudgetMaxMS:        30000,
			DisableSSRFProtection:     true,
			SimEngineBaseURL:          simEngine.URL,
			SimEngineInternalKey:      "sim-key",
		},
		zerolog.Nop(),
	)

	resp, err := svc.Run(context.Background(), orgID, gwdomain.RunRequest{
		RequestID: "req-world-rate-1",
		ToolName:  "world.observe",
		Input: map[string]any{
			"org_id":   orgID.String(),
			"run_id":   "run-2",
			"agent_id": "agent-002",
			"step_id":  8,
		},
		Context: map[string]any{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Status != gwdomain.RunStatusBlocked {
		t.Fatalf("expected blocked got %s", resp.Status)
	}
	if got == nil {
		t.Fatalf("expected enforcement payload")
	}
	if got["event_type"] != "tool.rate_limited" {
		t.Fatalf("expected tool.rate_limited got %#v", got["event_type"])
	}
	if got["bucket"] != "org+tool" {
		t.Fatalf("expected bucket org+tool got %#v", got["bucket"])
	}
}
