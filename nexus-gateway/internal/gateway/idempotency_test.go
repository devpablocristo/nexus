package gateway

import (
	"context"
	"testing"
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
	"nexus-gateway/pkg/validations/jsonschema"
)

type fakeToolRepo struct{ tool tooldomain.Tool }

func (f fakeToolRepo) GetByName(context.Context, uuid.UUID, string) (tooldomain.Tool, error) {
	return f.tool, nil
}

type fakePolicyRepo struct{}

func (fakePolicyRepo) ListByToolID(context.Context, uuid.UUID, uuid.UUID) ([]policydomain.Policy, error) {
	return nil, nil
}

type fakeAuditRepo struct{}

func (fakeAuditRepo) Create(context.Context, auditdomain.AuditEvent) error { return nil }

type fakeSecretRepo struct{}

func (fakeSecretRepo) ListForTool(context.Context, uuid.UUID, uuid.UUID) ([]secretdomain.ToolSecret, error) {
	return nil, nil
}

type fakeEgress struct{}

func (fakeEgress) IsHostAllowed(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return true, nil
}

type fakeLimiter struct{}

func (fakeLimiter) Allow(string, int) bool { return true }

type fakeExecutor struct{ calls int }

func (f *fakeExecutor) Execute(context.Context, string, string, map[string]any, map[string]string, int) (any, int, *types.HTTPError) {
	f.calls++
	return map[string]any{"ok": true}, 200, nil
}

type fakeIdempotency struct {
	record *gwdomain.IdempotencyRecord
}

func (f fakeIdempotency) Get(context.Context, uuid.UUID, string, string) (*gwdomain.IdempotencyRecord, error) {
	return f.record, nil
}
func (f fakeIdempotency) CreateInProgress(context.Context, gwdomain.IdempotencyRecord) error {
	return nil
}
func (f fakeIdempotency) MarkCompleted(context.Context, uuid.UUID, string, string, map[string]any) error {
	return nil
}
func (f fakeIdempotency) MarkFailed(context.Context, uuid.UUID, string, string, *string, map[string]any) error {
	return nil
}
func (f fakeIdempotency) DeleteExpired(context.Context, time.Time) (int64, error) { return 0, nil }

type fakeMetrics struct{}

func (fakeMetrics) ObserveRun(context.Context, string, string, string, time.Duration) {}

func TestBuildRequestFingerprintStable(t *testing.T) {
	actor := "a1"
	role := "bot"
	fp1, err := buildRequestFingerprint("transfer", map[string]any{"b": 2, "a": 1}, &actor, &role, []string{"b", "a"})
	if err != nil {
		t.Fatalf("fingerprint1: %v", err)
	}
	fp2, err := buildRequestFingerprint("transfer", map[string]any{"a": 1, "b": 2}, &actor, &role, []string{"a", "b"})
	if err != nil {
		t.Fatalf("fingerprint2: %v", err)
	}
	if fp1 != fp2 {
		t.Fatalf("expected stable fingerprint, got %s vs %s", fp1, fp2)
	}
}

func TestRun_IdempotencyReplayDoesNotExecute(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	actor := "bot-1"
	role := "bot"
	scopes := []string{"tools:run"}
	input := map[string]any{"amount": 10.0}
	fp, err := buildRequestFingerprint("transfer", input, &actor, &role, scopes)
	if err != nil {
		t.Fatalf("build fp: %v", err)
	}

	exec := &fakeExecutor{}
	svc := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:         toolID,
			OrgID:      orgID,
			Name:       "transfer",
			Kind:       tooldomain.ToolKindHTTP,
			Method:     "POST",
			URL:        "http://mock/transfer",
			ActionType: tooldomain.ActionWrite,
			Enabled:    true,
		}},
		fakePolicyRepo{},
		fakeAuditRepo{},
		fakeSecretRepo{},
		fakeEgress{},
		fakeLimiter{},
		exec,
		fakeIdempotency{record: &gwdomain.IdempotencyRecord{
			OrgID:              orgID,
			ToolName:           "transfer",
			IdempotencyKey:     "k1",
			RequestFingerprint: fp,
			Status:             gwdomain.IdempotencyStatusCompleted,
			ResponseRedacted: map[string]any{
				"status":   "success",
				"decision": "allow",
				"result":   map[string]any{"ok": true},
			},
		}},
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{TimeoutBudgetDefaultMS: 10000, TimeoutBudgetMinMS: 1000, TimeoutBudgetMaxMS: 30000},
		zerolog.Nop(),
	)

	idk := "k1"
	resp, err := svc.Run(context.Background(), orgID, gwdomain.RunRequest{
		ToolName:       "transfer",
		Input:          input,
		Context:        map[string]any{},
		Actor:          &actor,
		Role:           &role,
		Scopes:         scopes,
		IdempotencyKey: &idk,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Idempotency.Outcome != gwdomain.IdempotencyReplay {
		t.Fatalf("expected replay got %s", resp.Idempotency.Outcome)
	}
	if exec.calls != 0 {
		t.Fatalf("expected no executor calls on replay, got %d", exec.calls)
	}
}

func TestRun_IdempotencyConflict(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	actor := "bot-1"
	role := "bot"
	svc := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:         toolID,
			OrgID:      orgID,
			Name:       "transfer",
			Kind:       tooldomain.ToolKindHTTP,
			Method:     "POST",
			URL:        "http://mock/transfer",
			ActionType: tooldomain.ActionWrite,
			Enabled:    true,
		}},
		fakePolicyRepo{},
		fakeAuditRepo{},
		fakeSecretRepo{},
		fakeEgress{},
		fakeLimiter{},
		&fakeExecutor{},
		fakeIdempotency{record: &gwdomain.IdempotencyRecord{
			OrgID:              orgID,
			ToolName:           "transfer",
			IdempotencyKey:     "k1",
			RequestFingerprint: "other",
			Status:             gwdomain.IdempotencyStatusCompleted,
		}},
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{TimeoutBudgetDefaultMS: 10000, TimeoutBudgetMinMS: 1000, TimeoutBudgetMaxMS: 30000},
		zerolog.Nop(),
	)

	idk := "k1"
	resp, err := svc.Run(context.Background(), orgID, gwdomain.RunRequest{
		ToolName:       "transfer",
		Input:          map[string]any{"amount": 10.0},
		Context:        map[string]any{},
		Actor:          &actor,
		Role:           &role,
		IdempotencyKey: &idk,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.HTTPStatus != 409 {
		t.Fatalf("expected 409 got %d", resp.HTTPStatus)
	}
	if resp.ErrorCode == nil || *resp.ErrorCode != types.ErrCodeIdempotencyConflict {
		t.Fatalf("expected conflict code got %+v", resp.ErrorCode)
	}
}

func TestRun_IdempotencyFailedReplayReturnsCachedError(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	actor := "bot-1"
	role := "bot"
	scopes := []string{"tools:run"}
	input := map[string]any{"amount": 10.0}
	fp, err := buildRequestFingerprint("transfer", input, &actor, &role, scopes)
	if err != nil {
		t.Fatalf("build fp: %v", err)
	}

	exec := &fakeExecutor{}
	svc := NewService(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:         toolID,
			OrgID:      orgID,
			Name:       "transfer",
			Kind:       tooldomain.ToolKindHTTP,
			Method:     "POST",
			URL:        "http://mock/transfer",
			ActionType: tooldomain.ActionWrite,
			Enabled:    true,
		}},
		fakePolicyRepo{},
		fakeAuditRepo{},
		fakeSecretRepo{},
		fakeEgress{},
		fakeLimiter{},
		exec,
		fakeIdempotency{record: &gwdomain.IdempotencyRecord{
			OrgID:              orgID,
			ToolName:           "transfer",
			IdempotencyKey:     "k-failed",
			RequestFingerprint: fp,
			Status:             gwdomain.IdempotencyStatusFailed,
			ErrorCode:          ptr(types.ErrCodeUpstream5xx),
			ResponseRedacted: map[string]any{
				"status":      "error",
				"decision":    "allow",
				"http_status": float64(502),
				"error": map[string]any{
					"code":    types.ErrCodeUpstream5xx,
					"message": "upstream 5xx",
				},
			},
		}},
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		Config{TimeoutBudgetDefaultMS: 10000, TimeoutBudgetMinMS: 1000, TimeoutBudgetMaxMS: 30000},
		zerolog.Nop(),
	)

	idk := "k-failed"
	resp, err := svc.Run(context.Background(), orgID, gwdomain.RunRequest{
		ToolName:       "transfer",
		Input:          input,
		Context:        map[string]any{},
		Actor:          &actor,
		Role:           &role,
		Scopes:         scopes,
		IdempotencyKey: &idk,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Idempotency.Outcome != gwdomain.IdempotencyReplay {
		t.Fatalf("expected replay got %s", resp.Idempotency.Outcome)
	}
	if resp.Status != gwdomain.RunStatusError {
		t.Fatalf("expected error status got %s", resp.Status)
	}
	if resp.ErrorCode == nil || *resp.ErrorCode != types.ErrCodeUpstream5xx {
		t.Fatalf("expected upstream error code got %+v", resp.ErrorCode)
	}
	if exec.calls != 0 {
		t.Fatalf("expected no executor calls on failed replay, got %d", exec.calls)
	}
}

