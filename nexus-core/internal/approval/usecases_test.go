package approval

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "nexus-core/internal/approval/usecases/domain"
	"nexus-core/internal/gateway"
)

type stubRepo struct {
	created   domain.PendingApproval
	decided   bool
	decidedID uuid.UUID
	expired   int64
}

func (r *stubRepo) Create(_ context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	pa := domain.PendingApproval{
		ID:        uuid.New(),
		OrgID:     req.OrgID,
		ToolID:    req.ToolID,
		RequestID: req.RequestID,
		ToolName:  req.ToolName,
		Actor:     req.Actor,
		Reason:    req.Reason,
		Status:    domain.StatusPending,
		ExpiresAt: time.Now().Add(time.Duration(req.TTLSeconds) * time.Second),
		CreatedAt: time.Now(),
	}
	r.created = pa
	return pa, nil
}

func (r *stubRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error) {
	return domain.PendingApproval{ID: id, OrgID: orgID, Status: domain.StatusPending}, nil
}

func (r *stubRepo) ListPending(_ context.Context, _ uuid.UUID, _ int) ([]domain.PendingApproval, error) {
	if r.created.ID != uuid.Nil {
		return []domain.PendingApproval{r.created}, nil
	}
	return nil, nil
}

func (r *stubRepo) Decide(_ context.Context, _ uuid.UUID, id uuid.UUID, _ domain.Status, _ string) error {
	r.decided = true
	r.decidedID = id
	return nil
}

func (r *stubRepo) ExpireOld(_ context.Context) (int64, error) {
	return r.expired, nil
}

func TestRequestApproval_DefaultTTL(t *testing.T) {
	repo := &stubRepo{}
	svc := NewUsecases(repo)

	pa, err := svc.RequestApproval(context.Background(), domain.CreateRequest{
		OrgID:    uuid.New(),
		ToolName: "danger-tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pa.Status != domain.StatusPending {
		t.Errorf("expected pending, got %s", pa.Status)
	}
	if pa.ToolName != "danger-tool" {
		t.Errorf("expected danger-tool, got %s", pa.ToolName)
	}
}

func TestApproveReject(t *testing.T) {
	repo := &stubRepo{}
	svc := NewUsecases(repo)

	id := uuid.New()
	orgID := uuid.New()

	if err := svc.Approve(context.Background(), orgID, id, "admin@co"); err != nil {
		t.Fatal(err)
	}
	if !repo.decided || repo.decidedID != id {
		t.Error("expected Decide to be called")
	}

	repo.decided = false
	if err := svc.Reject(context.Background(), orgID, id, "admin@co"); err != nil {
		t.Fatal(err)
	}
	if !repo.decided {
		t.Error("expected Decide to be called for reject")
	}
}

func TestListPending(t *testing.T) {
	repo := &stubRepo{}
	svc := NewUsecases(repo)

	orgID := uuid.New()
	items, err := svc.ListPending(context.Background(), orgID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty, got %d", len(items))
	}
}

func TestGatewayAdapter(t *testing.T) {
	repo := &stubRepo{}
	svc := NewUsecases(repo)
	adapter := NewGatewayAdapter(svc)

	idStr, err := adapter.RequestApproval(context.Background(), gateway.ApprovalRequest{
		OrgID:    uuid.New(),
		ToolName: "test-tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if idStr == "" {
		t.Error("expected non-empty approval ID")
	}
}
