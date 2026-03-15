package alerts

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
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

	created, err := repo.Create(context.Background(), alertdomain.Alert{
		SourceKind: alertdomain.SourceKindIncident,
		SourceID:   "incident-1",
		Channel:    alertdomain.ChannelSlack,
		Route:      "ops-p2",
		Severity:   alertdomain.SeverityHigh,
		Status:     alertdomain.StatusPending,
		Summary:    "incident created",
		Body:       "body",
		Details:    map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	items, err := repo.List(context.Background(), ListFilters{SourceKind: "incident", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected created alert in list")
	}

	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse created id: %v", err)
	}

	created.Status = alertdomain.StatusAcknowledged
	updated, err := repo.Update(context.Background(), created)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Status != alertdomain.StatusAcknowledged {
		t.Fatalf("unexpected updated alert: %#v", updated)
	}

	archived, err := repo.Archive(context.Background(), id, nowUTC())
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("expected archived alert: %#v", archived)
	}

	restored, err := repo.Restore(context.Background(), id, nowUTC())
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("expected restored alert: %#v", restored)
	}
}
