package alerts

import (
	"context"
	"testing"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
)

type stubAlertAuditSink struct {
	items []sharedaudit.WriteRequest
}

func (s *stubAlertAuditSink) Create(_ context.Context, req sharedaudit.WriteRequest) error {
	s.items = append(s.items, req)
	return nil
}

type stubAlertMetrics struct {
	created []string
}

func (s *stubAlertMetrics) IncAlertCreated(channel, severity string) {
	s.created = append(s.created, channel+":"+severity)
}

func validCreateRequest() CreateRequest {
	return CreateRequest{
		SourceKind:   alertdomain.SourceKindIncident,
		SourceID:     "incident-1",
		ActionID:     "action-1",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Channel:      alertdomain.ChannelSlack,
		Route:        "ops-p2",
		Severity:     alertdomain.SeverityHigh,
		Summary:      "withdrawal blocked by Nexus",
		Body:         "incident requires operator attention",
		Details:      map[string]any{"incident_id": "incident-1"},
	}
}

func TestUsecasesCreate(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	item, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if item.ID == "" || item.Status != alertdomain.StatusPending {
		t.Fatalf("unexpected created alert: %#v", item)
	}
	if item.Details["incident_id"] != "incident-1" || item.Details["action_id"] != "action-1" {
		t.Fatalf("unexpected alert details: %#v", item.Details)
	}
}

func TestUsecasesLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	created, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	items, err := uc.List(context.Background(), ListRequest{Channel: "slack", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", items)
	}

	updated, err := uc.UpdateByID(context.Background(), mustAlertID(t, created.ID), UpdateRequest{
		Status:  ptr(alertdomain.StatusAcknowledged),
		Summary: strPtr("acknowledged by operator"),
	})
	if err != nil {
		t.Fatalf("UpdateByID returned error: %v", err)
	}
	if updated.Status != alertdomain.StatusAcknowledged || updated.Summary != "acknowledged by operator" {
		t.Fatalf("unexpected updated alert: %#v", updated)
	}

	archived, err := uc.ArchiveByID(context.Background(), mustAlertID(t, created.ID))
	if err != nil {
		t.Fatalf("ArchiveByID returned error: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("expected archived alert: %#v", archived)
	}

	restored, err := uc.RestoreByID(context.Background(), mustAlertID(t, created.ID))
	if err != nil {
		t.Fatalf("RestoreByID returned error: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("expected restored alert: %#v", restored)
	}

	if err := uc.DeleteByID(context.Background(), mustAlertID(t, created.ID)); err != nil {
		t.Fatalf("DeleteByID returned error: %v", err)
	}
}

func TestUsecasesCreateEmitsAudit(t *testing.T) {
	t.Parallel()

	audits := &stubAlertAuditSink{}
	uc := NewUsecases(NewInMemoryRepository(nil)).WithAuditSink(audits)

	item, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if item.ID == "" {
		t.Fatal("expected alert id")
	}
	if len(audits.items) != 1 || audits.items[0].EventType != "alert_created" {
		t.Fatalf("unexpected audit payloads: %#v", audits.items)
	}
	if audits.items[0].IncidentID != "incident-1" || audits.items[0].AlertID != item.ID || audits.items[0].ActionID != "action-1" {
		t.Fatalf("unexpected audit correlation fields: %#v", audits.items)
	}
}

func TestUsecasesCreateEmitsMetrics(t *testing.T) {
	t.Parallel()

	metrics := &stubAlertMetrics{}
	uc := NewUsecases(NewInMemoryRepository(nil)).WithMetrics(metrics)

	_, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if len(metrics.created) != 1 || metrics.created[0] != "slack:high" {
		t.Fatalf("unexpected metrics payloads: %#v", metrics.created)
	}
}

func mustAlertID(t *testing.T, raw string) [16]byte {
	t.Helper()
	id := parseUUID(t, raw)
	return id
}
