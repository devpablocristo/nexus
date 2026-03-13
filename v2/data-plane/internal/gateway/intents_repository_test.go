package gateway

import (
	"context"
	"testing"

	"github.com/google/uuid"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
)

func TestInMemoryIntentRepositoryCreateAndLinkApproval(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryIntentRepository()
	intent, err := repo.Create(context.Background(), gwdomain.ExecutionIntent{
		ToolID:    "tool_echo",
		ToolName:  "echo",
		RequestID: "req-123",
		Status:    gwdomain.IntentStatusPendingApproval,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if intent.ID == uuid.Nil {
		t.Fatal("expected intent id")
	}

	approvalID := uuid.New()
	if err := repo.LinkApproval(context.Background(), intent.ID, approvalID); err != nil {
		t.Fatalf("LinkApproval returned error: %v", err)
	}

	stored, ok := repo.items[intent.ID]
	if !ok {
		t.Fatal("expected stored intent")
	}
	if stored.ApprovalID == nil || *stored.ApprovalID != approvalID {
		t.Fatalf("unexpected approval id: %#v", stored.ApprovalID)
	}

	if err := repo.MarkApproved(context.Background(), intent.ID); err != nil {
		t.Fatalf("MarkApproved returned error: %v", err)
	}
	stored = repo.items[intent.ID]
	if stored.Status != gwdomain.IntentStatusApproved {
		t.Fatalf("unexpected approved status: %s", stored.Status)
	}

	if err := repo.MarkRejected(context.Background(), intent.ID); err != nil {
		t.Fatalf("MarkRejected returned error: %v", err)
	}
	stored = repo.items[intent.ID]
	if stored.Status != gwdomain.IntentStatusRejected {
		t.Fatalf("unexpected rejected status: %s", stored.Status)
	}

	got, err := repo.GetByID(context.Background(), intent.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if got.ID != intent.ID {
		t.Fatalf("unexpected intent from GetByID: %#v", got)
	}
}

func TestInMemoryIntentRepositoryListRecent(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryIntentRepository()
	first, err := repo.Create(context.Background(), gwdomain.ExecutionIntent{
		ToolID:    "tool_a",
		ToolName:  "a",
		RequestID: "req-a",
		Status:    gwdomain.IntentStatusPendingApproval,
	})
	if err != nil {
		t.Fatalf("Create first returned error: %v", err)
	}
	second, err := repo.Create(context.Background(), gwdomain.ExecutionIntent{
		ToolID:    "tool_b",
		ToolName:  "b",
		RequestID: "req-b",
		Status:    gwdomain.IntentStatusPendingApproval,
	})
	if err != nil {
		t.Fatalf("Create second returned error: %v", err)
	}

	items, err := repo.ListRecent(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected item count: %d", len(items))
	}
	if items[0].ID != second.ID && items[0].ID != first.ID {
		t.Fatalf("unexpected intent in recent list: %#v", items[0])
	}
}

func TestInMemoryIntentRepositoryMarkExecuted(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryIntentRepository()
	intent, err := repo.Create(context.Background(), gwdomain.ExecutionIntent{
		ToolID:    "tool_echo",
		ToolName:  "echo",
		RequestID: "req-123",
		Status:    gwdomain.IntentStatusApproved,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := repo.MarkExecuted(context.Background(), intent.ID); err != nil {
		t.Fatalf("MarkExecuted returned error: %v", err)
	}

	stored, err := repo.GetByID(context.Background(), intent.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if stored.Status != gwdomain.IntentStatusExecuted {
		t.Fatalf("unexpected executed status: %s", stored.Status)
	}
	if stored.ExecutedAt == nil {
		t.Fatalf("expected executed_at to be set: %#v", stored)
	}
}
