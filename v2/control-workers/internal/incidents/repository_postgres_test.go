package incidents

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	incidentdomain "nexus/v2/control-workers/internal/incidents/usecases/domain"
)

func TestPostgresRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("NEXUS_TEST_CONTROL_WORKERS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("NEXUS_TEST_CONTROL_WORKERS_DATABASE_URL not set")
	}

	repo, cleanup, err := NewPostgresRepository(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresRepository returned error: %v", err)
	}
	defer cleanup()

	created, err := repo.Create(context.Background(), incidentdomain.Incident{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerBlockedAction,
		RiskLevel:    incidentdomain.RiskLevelHigh,
		Severity:     incidentdomain.SeverityHigh,
		Status:       incidentdomain.StatusOpen,
		Summary:      "withdrawal blocked",
		Reason:       "blocked",
		Details:      map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	items, err := repo.List(context.Background(), ListFilters{SourceKind: "action", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected created incident in list")
	}

	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse created id: %v", err)
	}

	created.Status = incidentdomain.StatusResolved
	updated, err := repo.Update(context.Background(), created)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Status != incidentdomain.StatusResolved {
		t.Fatalf("unexpected updated incident: %#v", updated)
	}

	archived, err := repo.Archive(context.Background(), id, nowUTC())
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("expected archived incident: %#v", archived)
	}

	restored, err := repo.Restore(context.Background(), id, nowUTC())
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("expected restored incident: %#v", restored)
	}
}
