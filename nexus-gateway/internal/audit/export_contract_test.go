package audit

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	auditdomain "nexus-gateway/internal/audit/usecases/domain"
	"nexus-gateway/pkg/types"
)

type exportContractService struct {
	items []auditdomain.AuditEvent
}

func (s exportContractService) Query(context.Context, uuid.UUID, auditdomain.Query) ([]auditdomain.AuditEvent, error) {
	return nil, errors.New("not implemented")
}

func (s exportContractService) StreamByFilters(_ context.Context, _ uuid.UUID, q auditdomain.Query, _ int, fn func(auditdomain.AuditEvent) error) error {
	for _, item := range s.items {
		if q.ToolName != nil && item.ToolName != *q.ToolName {
			continue
		}
		if q.Decision != nil && item.Decision != *q.Decision {
			continue
		}
		if q.Status != nil && item.Status != *q.Status {
			continue
		}
		if q.From != nil && item.CreatedAt.Before(*q.From) {
			continue
		}
		if q.To != nil && item.CreatedAt.After(*q.To) {
			continue
		}
		if err := fn(item); err != nil {
			return err
		}
	}
	return nil
}

func TestAuditExportJSONLContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	router := buildAuditExportRouter(orgID)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/export?format=jsonl&tool_name=transfer&limit=10", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	lines := strings.Split(strings.TrimSpace(rr.Body.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines got %d body=%s", len(lines), rr.Body.String())
	}
	for _, line := range lines {
		if !strings.Contains(line, `"request_id"`) || !strings.Contains(line, `"event_hash"`) || !strings.Contains(line, `"tool_name":"transfer"`) {
			t.Fatalf("jsonl contract missing required fields: %s", line)
		}
	}
	assertGolden(t, "export.jsonl.golden", rr.Body.Bytes())
}

func TestAuditExportCSVContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	router := buildAuditExportRouter(orgID)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/export?format=csv&tool_name=transfer&limit=10", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.HasPrefix(body, "created_at,org_id,request_id,tool_name,actor,role") {
		t.Fatalf("missing csv header: %s", body)
	}
	if !strings.Contains(body, ",hash_2,sha256") {
		t.Fatalf("csv contract missing hash chain fields: %s", body)
	}
	assertGolden(t, "export.csv.golden", rr.Body.Bytes())
}

func buildAuditExportRouter(orgID uuid.UUID) *gin.Engine {
	actor := "agent-1"
	role := "bot"
	timeout := 10000
	budget := 8800
	events := []auditdomain.AuditEvent{
		{
			ID:                         uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			OrgID:                      orgID,
			ToolID:                     uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			ToolName:                   "transfer",
			RequestID:                  "req_1",
			Actor:                      &actor,
			ActorRole:                  &role,
			ActorScopes:                []string{"tools:run"},
			InputRedacted:              map[string]any{"amount": 500, "card_number": "***"},
			ContextRedacted:            map[string]any{"user_id": "u1"},
			DLPSummary:                 map[string]any{"credit_card": map[string]any{"count": 1}},
			Decision:                   auditdomain.DecisionDeny,
			Reason:                     strPtr("policy denied"),
			Status:                     auditdomain.StatusBlocked,
			ErrorCode:                  strPtr(types.ErrCodePolicyDenied),
			ErrorMessage:               strPtr("policy denied"),
			LatencyMS:                  7,
			IdempotencyPresent:         true,
			IdempotencyOutcome:         "NEW",
			TimeoutMS:                  &timeout,
			BudgetRemainingMSAtExecute: &budget,
			StageDurationsMS:           map[string]int64{"schema_validation": 1},
			PrevEventHash:              nil,
			EventHash:                  strPtr("hash_1"),
			CreatedAt:                  time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC),
		},
		{
			ID:                         uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
			OrgID:                      orgID,
			ToolID:                     uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			ToolName:                   "transfer",
			RequestID:                  "req_2",
			Actor:                      &actor,
			ActorRole:                  &role,
			ActorScopes:                []string{"tools:run"},
			InputRedacted:              map[string]any{"amount": 500},
			ContextRedacted:            map[string]any{"user_id": "u1"},
			DLPSummary:                 map[string]any{"credit_card": map[string]any{"count": 0}},
			Decision:                   auditdomain.DecisionAllow,
			Status:                     auditdomain.StatusSuccess,
			OutputRedacted:             map[string]any{"ok": true, "tx_id": "tx_1"},
			LatencyMS:                  33,
			IdempotencyPresent:         true,
			IdempotencyOutcome:         "REPLAY",
			TimeoutMS:                  &timeout,
			BudgetRemainingMSAtExecute: &budget,
			StageDurationsMS:           map[string]int64{"execute_http": 28},
			PrevEventHash:              strPtr("hash_1"),
			EventHash:                  strPtr("hash_2"),
			CreatedAt:                  time.Date(2026, 1, 10, 8, 0, 3, 0, time.UTC),
		},
	}

	h := NewHandler(exportContractService{items: events})
	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), orgID)
		c.Next()
	})
	h.Register(v1)
	return r
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
		t.Fatalf("golden mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", name, string(want), string(got))
	}
}

func strPtr(v string) *string { return &v }
