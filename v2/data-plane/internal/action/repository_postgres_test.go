package action

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

func TestPostgresRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("NEXUS_TEST_DATA_PLANE_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("NEXUS_TEST_DATA_PLANE_DATABASE_URL not set")
	}

	repo, cleanup, err := NewPostgresRepository(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresRepository returned error: %v", err)
	}
	defer cleanup()

	now := time.Now().UTC()
	actionID := uuid.New()
	created, err := repo.Create(context.Background(), actiondomain.Action{
		ID:            actionID,
		Type:          actiondomain.ActionTypeWithdrawal,
		Status:        actiondomain.ActionStatusPendingApproval,
		Decision:      actiondomain.DecisionRequireApproval,
		ResourceID:    "wallet_hot_usdc_1",
		ResourceType:  actiondomain.ResourceTypeWallet,
		SourceSystem:  "treasury-orchestrator",
		Justification: "Daily settlement withdrawal",
		RequestedBy:   actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "treasury-bot"},
		ProposedBy:    actiondomain.ActorRef{Type: actiondomain.ActorTypeAgent, ID: "treasury-agent"},
		Payload:       []byte(`{"asset":"USDC","amount":"25"}`),
		Metadata:      map[string]any{"ticket_id": "CHG-1234"},
		Risk: actiondomain.RiskAssessment{
			Level:   actiondomain.RiskLevelHigh,
			Score:   80,
			Summary: "requires approval",
		},
		Evidence: []actiondomain.EvidenceRecord{{
			ID:        uuid.New(),
			ActionID:  actionID,
			Kind:      "payload_validation",
			Status:    actiondomain.EvidenceStatusPassed,
			Summary:   "payload ok",
			CreatedAt: now,
		}},
		Approval: &actiondomain.Approval{
			ID:            uuid.New(),
			ActionID:      actionID,
			Status:        actiondomain.ApprovalStatusPending,
			RequiredCount: 1,
			ExpiresAt:     now.Add(time.Hour),
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	items, err := repo.List(context.Background(), ListFilters{ActionType: "withdrawal", Status: "pending_approval", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected created action in list")
	}

	approved, err := repo.Decide(context.Background(), created.ID, actiondomain.ApprovalStatusApproved, actiondomain.ActorRef{
		Type: actiondomain.ActorTypeUser,
		ID:   "alice",
	}, "approved", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if approved.Status != actiondomain.ActionStatusApproved {
		t.Fatalf("unexpected approved status: %s", approved.Status)
	}

	leased, err := repo.IssueLease(context.Background(), created.ID, actiondomain.ExecutionLease{
		ID:        uuid.New(),
		ActionID:  created.ID,
		Status:    actiondomain.LeaseStatusActive,
		Scope:     actiondomain.LeaseScope{ActionID: created.ID, ActionType: created.Type, ResourceID: created.ResourceID, ResourceType: created.ResourceType},
		ExpiresAt: now.Add(2 * time.Minute),
		CreatedAt: now.Add(90 * time.Second),
	})
	if err != nil {
		t.Fatalf("IssueLease returned error: %v", err)
	}

	executed, err := repo.ConsumeLeaseAndMarkExecuted(context.Background(), created.ID, leased.Lease.ID, actiondomain.ExecutionResult{
		Status:     "success",
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "wallet-orchestrator"},
		Result:     map[string]any{"execution_id": "exec_1"},
		ExecutedAt: now.Add(100 * time.Second),
	})
	if err != nil {
		t.Fatalf("ConsumeLeaseAndMarkExecuted returned error: %v", err)
	}
	if executed.Status != actiondomain.ActionStatusExecuted {
		t.Fatalf("unexpected executed status: %s", executed.Status)
	}
}
