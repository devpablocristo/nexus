package resources

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	resourcedomain "nexus/v2/control-plane/internal/resources/usecases/domain"
)

func TestPostgresRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("NEXUS_TEST_CONTROL_PLANE_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("NEXUS_TEST_CONTROL_PLANE_DATABASE_URL not set")
	}

	repo, cleanup, err := NewPostgresRepository(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresRepository returned error: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	created, err := repo.Create(ctx, resourcedomain.ProtectedResource{
		Type:        resourcedomain.ResourceTypeWallet,
		Name:        "wallet hot usdc 1",
		Environment: "prod",
		Chain:       "ethereum",
		Labels:      map[string]string{"tier": "hot"},
		Criticality: resourcedomain.CriticalityCritical,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse created id: %v", err)
	}

	items, err := repo.List(ctx, ListFilters{Archived: ptrBool(false), Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected created resource in list")
	}

	created.Name = "wallet hot usdc primary"
	created.Chain = "base"
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Name != "wallet hot usdc primary" || updated.Chain != "base" {
		t.Fatalf("unexpected updated resource: %#v", updated)
	}

	archived, err := repo.Archive(ctx, id, nowUTC())
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("expected archived resource: %#v", archived)
	}

	restored, err := repo.Restore(ctx, id, nowUTC())
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("expected restored resource: %#v", restored)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := repo.GetByID(ctx, id); err == nil {
		t.Fatal("expected deleted resource to be missing")
	}
}

func ptrBool(value bool) *bool {
	return &value
}
