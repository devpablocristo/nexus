package tool

import (
	"context"
	"testing"
)

func TestInMemoryRepositoryResolvesByIDAndName(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepository([]Definition{
		{ID: "tool_echo", Name: "echo", Enabled: true},
		{Name: "fallback-id", Enabled: true},
	})

	byID, err := repo.GetByID(context.Background(), "tool_echo")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if byID.Name != "echo" {
		t.Fatalf("unexpected definition by id: %#v", byID)
	}

	byName, err := repo.GetByName(context.Background(), "fallback-id")
	if err != nil {
		t.Fatalf("GetByName returned error: %v", err)
	}
	if byName.ID != "fallback-id" {
		t.Fatalf("expected fallback id, got %#v", byName)
	}
}
