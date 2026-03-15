package alerts

import (
	"context"
	"testing"

	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
)

func validCreateRequest() CreateRequest {
	return CreateRequest{
		SourceKind: alertdomain.SourceKindIncident,
		SourceID:   "incident-1",
		Channel:    alertdomain.ChannelSlack,
		Route:      "ops-p2",
		Severity:   alertdomain.SeverityHigh,
		Summary:    "withdrawal blocked by Nexus",
		Body:       "incident requires operator attention",
		Details:    map[string]any{"incident_id": "incident-1"},
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

func mustAlertID(t *testing.T, raw string) [16]byte {
	t.Helper()
	id := parseUUID(t, raw)
	return id
}
