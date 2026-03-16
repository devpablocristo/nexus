package resources

import (
	"context"
	"testing"

	"github.com/google/uuid"

	resourcedomain "nexus/v2/control-plane/internal/resources/usecases/domain"
)

func TestUsecasesResourceLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	created, err := uc.Create(context.Background(), CreateRequest{
		Type:        resourcedomain.ResourceTypeWallet,
		Name:        "wallet hot usdc 1",
		Environment: "prod",
		Chain:       "ethereum",
		Labels:      map[string]string{"tier": "hot"},
		Criticality: resourcedomain.CriticalityCritical,
		IsCanary:    true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected resource id")
	}
	if !created.IsCanary || created.Labels["_nexus_trap"] != "true" {
		t.Fatalf("expected canary labeling on create: %#v", created)
	}

	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("parse created id: %v", err)
	}

	updated, err := uc.UpdateByID(context.Background(), id, UpdateRequest{
		Name:     ptr("wallet hot usdc primary"),
		Chain:    ptr("base"),
		IsCanary: ptrBool(false),
	})
	if err != nil {
		t.Fatalf("UpdateByID returned error: %v", err)
	}
	if updated.Name != "wallet hot usdc primary" || updated.Chain != "base" {
		t.Fatalf("unexpected updated resource: %#v", updated)
	}
	if updated.IsCanary || updated.Labels["_nexus_trap"] != "" {
		t.Fatalf("expected canary labeling removed on update: %#v", updated)
	}

	archived, err := uc.ArchiveByID(context.Background(), id)
	if err != nil {
		t.Fatalf("ArchiveByID returned error: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("expected archived resource: %#v", archived)
	}

	items, err := uc.List(context.Background(), ListRequest{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected archived resource to be hidden by default: %#v", items)
	}

	restored, err := uc.RestoreByID(context.Background(), id)
	if err != nil {
		t.Fatalf("RestoreByID returned error: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("expected restored resource: %#v", restored)
	}

	items, err = uc.List(context.Background(), ListRequest{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected listed resources: %#v", items)
	}

	if err := uc.DeleteByID(context.Background(), id); err != nil {
		t.Fatalf("DeleteByID returned error: %v", err)
	}

	if _, err := uc.GetByID(context.Background(), id); err == nil {
		t.Fatal("expected deleted resource to be missing")
	}
}

func TestUsecasesRejectsInvalidResourceType(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	_, err := uc.Create(context.Background(), CreateRequest{
		Type:        resourcedomain.ResourceType("keyset"),
		Name:        "wallet hot usdc 1",
		Environment: "prod",
		Chain:       "ethereum",
		Criticality: resourcedomain.CriticalityHigh,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func ptr(value string) *string {
	return &value
}
