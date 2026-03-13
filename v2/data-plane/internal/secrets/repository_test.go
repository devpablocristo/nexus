package secrets

import (
	"context"
	"testing"

	secretdomain "nexus/v2/data-plane/internal/secrets/usecases/domain"
)

func TestInMemoryRepositoryListForTool(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepository([]secretdomain.ToolSecret{
		{ToolID: "tool_echo", SecretType: "header", KeyName: "X-API-Key", PlaintextValue: "abc", Enabled: true},
		{ToolID: "tool_echo", SecretType: "bearer", PlaintextValue: "token", Enabled: true},
		{ToolID: "other", SecretType: "header", KeyName: "X-Other", PlaintextValue: "zzz", Enabled: true},
	})

	items, err := repo.ListForTool(context.Background(), "tool_echo")
	if err != nil {
		t.Fatalf("ListForTool returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: %d", len(items))
	}

	items[0].KeyName = "mutated"

	fresh, err := repo.ListForTool(context.Background(), "tool_echo")
	if err != nil {
		t.Fatalf("ListForTool returned error: %v", err)
	}
	if fresh[0].KeyName != "X-API-Key" {
		t.Fatalf("repository leaked internal slice mutation: %#v", fresh)
	}
}
