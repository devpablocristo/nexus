package audit

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
)

func TestUsecasesCreateAndList(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	created, err := uc.Create(context.Background(), sharedaudit.WriteRequest{
		EventType:     "action_created",
		SourceService: "data-plane",
		ActionID:      "action-1",
		IncidentID:    "incident-1",
		AlertID:       "alert-1",
		ResourceID:    "resource-1",
		ResourceType:  "wallet",
		Actor:         &sharedaudit.Actor{Type: "system", ID: "treasury-bot"},
		Summary:       "withdrawal created",
		Data:          map[string]any{"status": "pending_approval"},
		OccurredAt:    nowUTC(),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected audit id")
	}

	items, err := uc.List(context.Background(), ListRequest{
		ActionID:   "action-1",
		IncidentID: "incident-1",
		AlertID:    "alert-1",
		EventType:  "action_created",
		ActorID:    "treasury-bot",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected list items: %#v", items)
	}

	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse created id: %v", err)
	}
	got, err := uc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if got.Summary != "withdrawal created" {
		t.Fatalf("unexpected audit record: %#v", got)
	}
	if got.IncidentID != "incident-1" || got.AlertID != "alert-1" {
		t.Fatalf("unexpected correlation fields: %#v", got)
	}
}

func TestUsecasesRejectsBadRange(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	_, err := uc.List(context.Background(), ListRequest{
		From:  time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		To:    time.Date(2026, 3, 15, 11, 0, 0, 0, time.UTC),
		Limit: 10,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
