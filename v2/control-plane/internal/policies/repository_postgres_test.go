package policies

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	policydomain "nexus/v2/control-plane/internal/policies/usecases/domain"
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
	created, err := repo.Create(ctx, policydomain.Policy{
		ActionType:         "withdrawal",
		ResourceType:       "wallet",
		Effect:             policydomain.EffectAllow,
		Priority:           10,
		Expression:         `action.action_type == "withdrawal" && resource.type == "wallet"`,
		Reason:             "withdrawals from wallets require approval",
		RequireApproval:    true,
		ApprovalTTLSeconds: 600,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	items, err := repo.List(ctx, ListFilters{ActionType: "withdrawal", ResourceType: "wallet"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected created policy in list")
	}

	created.Reason = "updated"
	saved, err := repo.Save(ctx, created)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if saved.Reason != "updated" {
		t.Fatalf("unexpected saved policy: %#v", saved)
	}

	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse created id: %v", err)
	}

	archived, err := repo.ArchiveByID(ctx, id)
	if err != nil {
		t.Fatalf("ArchiveByID returned error: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("expected archived policy: %#v", archived)
	}

	restored, err := repo.RestoreByID(ctx, id)
	if err != nil {
		t.Fatalf("RestoreByID returned error: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("expected restored policy: %#v", restored)
	}

	if err := repo.DeleteByID(ctx, id); err != nil {
		t.Fatalf("DeleteByID returned error: %v", err)
	}
	if _, err := repo.GetByID(ctx, id); err == nil {
		t.Fatal("expected deleted policy to be missing")
	}
}
