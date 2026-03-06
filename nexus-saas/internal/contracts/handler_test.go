package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-saas/cmd/config"
	actionsvc "nexus-saas/internal/actions"
	actiondomain "nexus-saas/internal/actions/usecases/domain"
	admindomain "nexus-saas/internal/admin/usecases/domain"
	billingdomain "nexus-saas/internal/billing/usecases/domain"
	eventdomain "nexus-saas/internal/events/usecases/domain"
	incidentdomain "nexus-saas/internal/incidents/usecases/domain"
	proposaldomain "nexus-saas/internal/policyproposal/usecases/domain"
)

type entitlementsStub struct {
	settings admindomain.TenantSettings
	ok       bool
	err      error
}

func (s *entitlementsStub) GetTenantSettings(_ context.Context, _ uuid.UUID) (admindomain.TenantSettings, bool, error) {
	return s.settings, s.ok, s.err
}

type usageStub struct {
	seen map[string]int
}

func (s *usageStub) IncrementEvent(_ context.Context, eventID string, _ uuid.UUID, _ string) error {
	if s.seen == nil {
		s.seen = map[string]int{}
	}
	s.seen[eventID]++
	return nil
}

func (s *usageStub) GetCounter(_ context.Context, _ uuid.UUID, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

type billingStub struct {
	tenant billingdomain.TenantBilling
	found  bool
	usage  billingdomain.UsageSummary
}

func (s *billingStub) GetTenantBilling(_ context.Context, _ uuid.UUID) (billingdomain.TenantBilling, bool, error) {
	return s.tenant, s.found, nil
}

func (s *billingStub) GetUsageSummary(_ context.Context, _ uuid.UUID, _ time.Time) (billingdomain.UsageSummary, error) {
	return s.usage, nil
}

type actionsStub struct {
	items     []actiondomain.Action
	overrides actiondomain.RuntimeOverrides
}

func (s *actionsStub) List(_ context.Context, _ uuid.UUID, _ actionsvc.ListQuery) ([]actiondomain.Action, error) {
	return s.items, nil
}

func (s *actionsStub) ResolveRuntimeOverrides(_ context.Context, _ uuid.UUID, _ string) (actiondomain.RuntimeOverrides, error) {
	return s.overrides, nil
}

type incidentsStub struct {
	items []incidentdomain.Incident
}

func (s *incidentsStub) List(_ context.Context, _ uuid.UUID, _ string, _ int) ([]incidentdomain.Incident, error) {
	return s.items, nil
}

type proposalsStub struct {
	items []proposaldomain.Proposal
}

func (s *proposalsStub) List(_ context.Context, _ uuid.UUID, _ string, _ int) ([]proposaldomain.Proposal, error) {
	return s.items, nil
}

type eventsStub struct {
	items []eventdomain.Event
}

func (s *eventsStub) Append(_ context.Context, _ uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error) {
	return eventdomain.Event{ID: 1, EventType: eventType, Payload: payload}, nil
}

func (s *eventsStub) ListRecent(_ context.Context, _ uuid.UUID, _ int) ([]eventdomain.Event, error) {
	return s.items, nil
}

func newRouterForTest(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterInternal(r)
	return r
}

func TestInternalEndpoints_RequireKey(t *testing.T) {
	h := &Handler{
		cfg:      config.ServiceConfig{SaaSInternalKey: "k1"},
		admin:    &entitlementsStub{},
		metering: &usageStub{},
	}
	r := newRouterForTest(h)
	req := httptest.NewRequest(http.MethodGet, "/internal/entitlements/"+uuid.NewString(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestIngestUsage_IsIdempotentByEventID(t *testing.T) {
	usage := &usageStub{}
	h := &Handler{
		cfg:      config.ServiceConfig{SaaSInternalKey: "k1"},
		admin:    &entitlementsStub{},
		metering: usage,
	}
	r := newRouterForTest(h)
	body := map[string]any{
		"event_id": "ev-1",
		"org_id":   uuid.NewString(),
		"counter":  "api_calls",
	}
	raw, _ := json.Marshal(body)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/internal/usage/events", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-NEXUS-SAAS-KEY", "k1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", w.Code)
		}
	}
	if usage.seen["ev-1"] != 2 {
		t.Fatalf("expected handler to pass event twice to sink, got %d", usage.seen["ev-1"])
	}
}

func TestGetAssistantContext_ReturnsTenantScopedSnapshot(t *testing.T) {
	orgID := uuid.New()
	scopeID := "payments-api"
	h := &Handler{
		cfg:   config.ServiceConfig{SaaSInternalKey: "k1"},
		admin: &entitlementsStub{ok: true, settings: admindomain.TenantSettings{OrgID: orgID, PlanCode: "growth", Status: admindomain.TenantStatusActive, HardLimits: map[string]any{"run_rpm": 1200}}},
		billing: &billingStub{
			found: true,
			tenant: billingdomain.TenantBilling{
				OrgID:         orgID,
				PlanCode:      billingdomain.PlanGrowth,
				BillingStatus: billingdomain.BillingActive,
			},
			usage: billingdomain.UsageSummary{
				Period: "2026-03",
				Counters: billingdomain.UsageCounters{
					APICalls:        12,
					EventsIngested:  4,
					IncidentsOpened: 1,
					ActionsExecuted: 2,
				},
			},
		},
		incidents: &incidentsStub{items: []incidentdomain.Incident{
			{
				ID:       uuid.New(),
				Severity: incidentdomain.SeverityHigh,
				Status:   incidentdomain.StatusOpen,
				Title:    "Deny spike",
				Summary:  "Cross-tenant deny ratio spike",
				OpenedAt: time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC),
			},
		}},
		actions: &actionsStub{items: []actiondomain.Action{
			{
				ID:         uuid.New(),
				ScopeType:  actiondomain.ScopeTool,
				ScopeID:    &scopeID,
				ActionType: actiondomain.ActionThrottleToolRPM,
				Status:     actiondomain.StatusActive,
				CreatedAt:  time.Date(2026, 3, 6, 12, 5, 0, 0, time.UTC),
			},
		}},
		proposals: &proposalsStub{items: []proposaldomain.Proposal{
			{
				ID:        uuid.New(),
				Status:    proposaldomain.StatusPending,
				Rationale: "Tighten rate limit",
				CreatedAt: time.Date(2026, 3, 6, 12, 10, 0, 0, time.UTC),
			},
		}},
		events: &eventsStub{items: []eventdomain.Event{
			{
				ID:        42,
				EventType: "incident.opened",
				CreatedAt: time.Date(2026, 3, 6, 12, 15, 0, 0, time.UTC),
				Payload:   map[string]any{"title": "Deny spike"},
			},
		}},
	}

	r := newRouterForTest(h)
	req := httptest.NewRequest(http.MethodGet, "/internal/assistant/context/"+orgID.String(), nil)
	req.Header.Set("X-NEXUS-SAAS-KEY", "k1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	tenant := body["tenant"].(map[string]any)
	if tenant["plan_code"] != "growth" {
		t.Fatalf("unexpected plan_code: %v", tenant["plan_code"])
	}
	billing := body["billing"].(map[string]any)
	if billing["billing_status"] != "active" {
		t.Fatalf("unexpected billing_status: %v", billing["billing_status"])
	}
	incidents := body["incidents"].(map[string]any)
	if incidents["open_count"].(float64) != 1 {
		t.Fatalf("unexpected open_count: %v", incidents["open_count"])
	}
	events := body["events"].(map[string]any)
	eventItems := events["items"].([]any)
	if len(eventItems) != 1 {
		t.Fatalf("expected one recent event, got %d", len(eventItems))
	}
	event := eventItems[0].(map[string]any)
	if event["summary"] == "" {
		t.Fatal("expected summarized recent event")
	}
}
