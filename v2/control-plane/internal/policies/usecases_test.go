package policies

import (
	"context"
	"testing"
)

func newTestPolicy() CreateRequest {
	return CreateRequest{
		ActionType:         "withdrawal",
		ResourceType:       "wallet",
		Effect:             "allow",
		Priority:           10,
		Expression:         `action.action_type == "withdrawal" && resource.type == "wallet"`,
		Reason:             "withdrawals from wallets require approval",
		RequireApproval:    true,
		ApprovalTTLSeconds: 600,
	}
}

func TestUsecasesPolicyLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil), NewEvaluator())

	created, err := uc.Create(context.Background(), newTestPolicy())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected policy id")
	}

	items, err := uc.List(context.Background(), ListRequest{ActionType: "withdrawal", ResourceType: "wallet"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", items)
	}
}
