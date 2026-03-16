package incidents

import (
	"context"
	"testing"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	"github.com/google/uuid"

	incidentdomain "nexus/v2/control-workers/internal/incidents/usecases/domain"
)

type stubIncidentAuditSink struct {
	items []sharedaudit.WriteRequest
}

func (s *stubIncidentAuditSink) Create(_ context.Context, req sharedaudit.WriteRequest) error {
	s.items = append(s.items, req)
	return nil
}

type stubIncidentMetrics struct {
	created []string
}

func (s *stubIncidentMetrics) IncIncidentCreated(trigger, severity string) {
	s.created = append(s.created, trigger+":"+severity)
}

func TestUsecasesCreateDerivesSeverityAndSummary(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	item, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerExecutionFailed,
		RiskLevel:    incidentdomain.RiskLevelCritical,
		Reason:       "executor could not reach signer",
		Details:      map[string]any{"attempt": 1},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if item.ID == "" {
		t.Fatal("expected incident id")
	}
	if item.Status != incidentdomain.StatusOpen {
		t.Fatalf("unexpected status: %s", item.Status)
	}
	if item.Severity != incidentdomain.SeverityCritical {
		t.Fatalf("unexpected severity: %s", item.Severity)
	}
	if item.Summary != "withdrawal failed during execution" {
		t.Fatalf("unexpected summary: %s", item.Summary)
	}
	if item.Details["action_id"] != "action-1" || item.Details["resource_id"] != "wallet_hot_usdc_1" {
		t.Fatalf("unexpected incident details: %#v", item.Details)
	}
}

func TestUsecasesUpdateCanResolveIncident(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	created, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerBlockedAction,
		RiskLevel:    incidentdomain.RiskLevelHigh,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	status := incidentdomain.StatusResolved
	summary := "withdrawal incident resolved after manual review"
	updated, err := uc.UpdateByID(context.Background(), mustUUID(t, created.ID), UpdateRequest{
		Status:  &status,
		Summary: &summary,
	})
	if err != nil {
		t.Fatalf("UpdateByID returned error: %v", err)
	}
	if updated.Status != incidentdomain.StatusResolved {
		t.Fatalf("unexpected status: %s", updated.Status)
	}
	if updated.ResolvedAt == nil {
		t.Fatal("expected resolved_at")
	}
}

func TestUsecasesCreateRejectsUnsupportedTrigger(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	_, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.Trigger("unexpected"),
		RiskLevel:    incidentdomain.RiskLevelHigh,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestUsecasesCreateEmitsAudit(t *testing.T) {
	t.Parallel()

	audits := &stubIncidentAuditSink{}
	uc := NewUsecases(NewInMemoryRepository(nil)).WithAuditSink(audits)

	item, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerBlockedAction,
		RiskLevel:    incidentdomain.RiskLevelHigh,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if item.ID == "" {
		t.Fatal("expected incident id")
	}
	if len(audits.items) != 1 || audits.items[0].EventType != "incident_created" {
		t.Fatalf("unexpected audit payloads: %#v", audits.items)
	}
	if audits.items[0].IncidentID != item.ID || audits.items[0].ActionID != "action-1" {
		t.Fatalf("unexpected audit correlation fields: %#v", audits.items)
	}
}

func TestUsecasesCreateEmitsMetrics(t *testing.T) {
	t.Parallel()

	metrics := &stubIncidentMetrics{}
	uc := NewUsecases(NewInMemoryRepository(nil)).WithMetrics(metrics)

	_, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerBlockedAction,
		RiskLevel:    incidentdomain.RiskLevelHigh,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if len(metrics.created) != 1 || metrics.created[0] != "blocked_action:high" {
		t.Fatalf("unexpected metrics payloads: %#v", metrics.created)
	}
}

func TestUsecasesCreateCanaryTriggerIsAlwaysCritical(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	item, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_canary_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerCanaryTriggered,
		RiskLevel:    incidentdomain.RiskLevelLow,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if item.Severity != incidentdomain.SeverityCritical {
		t.Fatalf("unexpected severity: %s", item.Severity)
	}
}

func mustUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	return id
}
