package approval

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "nexus/v2/data-plane/internal/approval/usecases/domain"
)

func TestRequestApprovalDefaultsTTL(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository())
	item, err := uc.RequestApproval(context.Background(), domain.CreateRequest{
		RequestID: "req-123",
		ToolName:  "echo",
		Reason:    "approval required",
	})
	if err != nil {
		t.Fatalf("RequestApproval returned error: %v", err)
	}
	if item.ID.String() == "" {
		t.Fatal("expected approval id")
	}
	if item.Status != domain.StatusPending {
		t.Fatalf("unexpected status: %s", item.Status)
	}
	if ttl := item.ExpiresAt.Sub(item.CreatedAt); ttl < 3590*time.Second || ttl > 3610*time.Second {
		t.Fatalf("unexpected ttl: %s", ttl)
	}
}

type intentStatusStub struct {
	approved []uuid.UUID
	rejected []uuid.UUID
}

func (s *intentStatusStub) MarkApproved(_ context.Context, intentID uuid.UUID) error {
	s.approved = append(s.approved, intentID)
	return nil
}

func (s *intentStatusStub) MarkRejected(_ context.Context, intentID uuid.UUID) error {
	s.rejected = append(s.rejected, intentID)
	return nil
}

func TestApproveAndRejectNotifyIntentPort(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepository()
	intentID := uuid.New()
	item, err := repo.Create(context.Background(), domain.CreateRequest{
		IntentID:   &intentID,
		RequestID:  "req-1",
		ToolName:   "echo",
		Reason:     "approval required",
		TTLSeconds: 60,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	stub := &intentStatusStub{}
	uc := NewUsecases(repo).WithIntentPort(stub)

	approved, err := uc.Approve(context.Background(), item.ID, "alice")
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if approved.Status != domain.StatusApproved {
		t.Fatalf("unexpected approved status: %s", approved.Status)
	}
	if approved.DecidedBy == nil || *approved.DecidedBy != "alice" {
		t.Fatalf("unexpected decided_by: %#v", approved.DecidedBy)
	}
	if len(stub.approved) != 1 || stub.approved[0] != intentID {
		t.Fatalf("intent port not notified on approve: %#v", stub.approved)
	}

	secondItem, err := repo.Create(context.Background(), domain.CreateRequest{
		IntentID:   &intentID,
		RequestID:  "req-2",
		ToolName:   "echo",
		Reason:     "approval required",
		TTLSeconds: 60,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	rejected, err := uc.Reject(context.Background(), secondItem.ID, "bob")
	if err != nil {
		t.Fatalf("Reject returned error: %v", err)
	}
	if rejected.Status != domain.StatusRejected {
		t.Fatalf("unexpected rejected status: %s", rejected.Status)
	}
	if rejected.DecidedBy == nil || *rejected.DecidedBy != "bob" {
		t.Fatalf("unexpected decided_by: %#v", rejected.DecidedBy)
	}
	if len(stub.rejected) != 1 || stub.rejected[0] != intentID {
		t.Fatalf("intent port not notified on reject: %#v", stub.rejected)
	}
}
