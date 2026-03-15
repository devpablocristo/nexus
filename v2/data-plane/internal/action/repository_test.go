package action

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

func TestInMemoryRepositoryConsumeLeaseAndMarkExecutedSingleUse(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	actionID := uuid.New()
	leaseID := uuid.New()
	repo := NewInMemoryRepository([]actiondomain.Action{{
		ID:            actionID,
		Type:          actiondomain.ActionTypeWithdrawal,
		Status:        actiondomain.ActionStatusLeased,
		Decision:      actiondomain.DecisionAllow,
		ResourceID:    "wallet_hot_usdc_1",
		ResourceType:  actiondomain.ResourceTypeWallet,
		SourceSystem:  "treasury-orchestrator",
		Justification: "Daily settlement withdrawal",
		RequestedBy:   actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "treasury-bot"},
		ProposedBy:    actiondomain.ActorRef{Type: actiondomain.ActorTypeAgent, ID: "treasury-agent"},
		Approval:      &actiondomain.Approval{ID: uuid.New(), ActionID: actionID, Status: actiondomain.ApprovalStatusApproved},
		Lease: &actiondomain.ExecutionLease{
			ID:        leaseID,
			ActionID:  actionID,
			Status:    actiondomain.LeaseStatusActive,
			Scope:     actiondomain.LeaseScope{ActionID: actionID, ActionType: actiondomain.ActionTypeWithdrawal, ResourceID: "wallet_hot_usdc_1", ResourceType: actiondomain.ResourceTypeWallet},
			ExpiresAt: now.Add(time.Minute),
			CreatedAt: now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}})

	first, err := repo.ConsumeLeaseAndMarkExecuted(context.Background(), actionID, leaseID, actiondomain.ExecutionResult{
		Status:     "success",
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "wallet-orchestrator"},
		Result:     map[string]any{"execution_id": "exec_" + actionID.String()},
		ExecutedAt: now.Add(10 * time.Second),
	})
	if err != nil {
		t.Fatalf("first execute returned error: %v", err)
	}
	if first.Status != actiondomain.ActionStatusExecuted {
		t.Fatalf("unexpected action status: %s", first.Status)
	}
	if first.Lease == nil || first.Lease.Status != actiondomain.LeaseStatusUsed {
		t.Fatalf("expected used lease: %#v", first.Lease)
	}

	_, err = repo.ConsumeLeaseAndMarkExecuted(context.Background(), actionID, leaseID, actiondomain.ExecutionResult{
		Status:     "success",
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "wallet-orchestrator"},
		Result:     map[string]any{"execution_id": "exec_" + actionID.String()},
		ExecutedAt: now.Add(20 * time.Second),
	})
	if err == nil {
		t.Fatal("expected second execute to fail")
	}
	if !errors.Is(err, ErrActionAlreadyExecuted) {
		t.Fatalf("expected ErrActionAlreadyExecuted, got %v", err)
	}
}
