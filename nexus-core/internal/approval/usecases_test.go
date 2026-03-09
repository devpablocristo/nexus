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
	created        domain.PendingApproval
	decided        bool
	decidedID      uuid.UUID
	expired        int64
	intentID       *uuid.UUID
	byIntent       []domain.PendingApproval
	rejectedIntent uuid.UUID
	getItem        *domain.PendingApproval
}

func (r *stubRepo) Create(_ context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	pa := domain.PendingApproval{
		ID:                 uuid.New(),
		OrgID:              req.OrgID,
		ToolID:             req.ToolID,
		IntentID:           req.IntentID,
		ApprovalMode:       req.ApprovalMode,
		ApprovalGroupID:    req.ApprovalGroupID,
		ApprovalStep:       req.ApprovalStep,
		ApprovalStepsTotal: req.ApprovalStepsTotal,
		RequestID:          req.RequestID,
		ToolName:           req.ToolName,
		Actor:              req.Actor,
		Reason:             req.Reason,
		Status:             domain.StatusPending,
		ExpiresAt:          time.Now().Add(time.Duration(req.TTLSeconds) * time.Second),
		CreatedAt:          time.Now(),
	}
	r.created = pa
	return pa, nil
}

func (r *stubRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error) {
	if r.getItem != nil {
		item := *r.getItem
		item.ID = id
		item.OrgID = orgID
		return item, nil
	}
	return domain.PendingApproval{ID: id, OrgID: orgID, Status: domain.StatusPending, IntentID: r.intentID}, nil
}

func (r *stubRepo) ListPending(_ context.Context, _ uuid.UUID, _ int) ([]domain.PendingApproval, error) {
	if r.created.ID != uuid.Nil {
		return []domain.PendingApproval{r.created}, nil
	}
	return nil, nil
}

func (r *stubRepo) ListByIntent(_ context.Context, _ uuid.UUID, _ uuid.UUID) ([]domain.PendingApproval, error) {
	return append([]domain.PendingApproval{}, r.byIntent...), nil
}

func (r *stubRepo) Decide(_ context.Context, _ uuid.UUID, id uuid.UUID, _ domain.Status, _ string) error {
	r.decided = true
	r.decidedID = id
	return nil
}

func (r *stubRepo) ExpireOld(_ context.Context) (int64, error) {
	return r.expired, nil
}

func (r *stubRepo) RejectPendingByIntent(_ context.Context, _ uuid.UUID, intentID uuid.UUID, _ string) error {
	r.rejectedIntent = intentID
	return nil
}

type stubIntentPort struct {
	approved uuid.UUID
	rejected uuid.UUID
}

func (s *stubIntentPort) MarkIntentApproved(_ context.Context, _ uuid.UUID, intentID uuid.UUID) error {
	s.approved = intentID
	return nil
}

func (s *stubIntentPort) MarkIntentRejected(_ context.Context, _ uuid.UUID, intentID uuid.UUID) error {
	s.rejected = intentID
	return nil
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
	if pa.ApprovalMode != domain.ApprovalModeStandard {
		t.Errorf("expected default approval mode standard, got %s", pa.ApprovalMode)
	}
	if pa.ApprovalStepsTotal != 1 {
		t.Errorf("expected single approval by default, got %d", pa.ApprovalStepsTotal)
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

func TestApproveRejectSyncsIntentStatus(t *testing.T) {
	intentID := uuid.New()
	repo := &stubRepo{intentID: &intentID}
	intentPort := &stubIntentPort{}
	svc := NewUsecases(repo).WithIntentPort(intentPort)

	approvalID := uuid.New()
	orgID := uuid.New()
	if err := svc.Approve(context.Background(), orgID, approvalID, "admin@co"); err != nil {
		t.Fatal(err)
	}
	if intentPort.approved != intentID {
		t.Fatalf("expected approved intent %s, got %s", intentID, intentPort.approved)
	}

	if err := svc.Reject(context.Background(), orgID, approvalID, "admin@co"); err != nil {
		t.Fatal(err)
	}
	if intentPort.rejected != intentID {
		t.Fatalf("expected rejected intent %s, got %s", intentID, intentPort.rejected)
	}
}

func TestBreakGlassApproveRequiresDistinctApprovers(t *testing.T) {
	intentID := uuid.New()
	groupID := uuid.New()
	repo := &stubRepo{
		intentID: &intentID,
		getItem: &domain.PendingApproval{
			IntentID:           &intentID,
			ApprovalMode:       domain.ApprovalModeBreakGlass,
			ApprovalGroupID:    &groupID,
			ApprovalStep:       2,
			ApprovalStepsTotal: 2,
			Status:             domain.StatusPending,
		},
		byIntent: []domain.PendingApproval{
			{
				ID:                 uuid.New(),
				IntentID:           &intentID,
				ApprovalMode:       domain.ApprovalModeBreakGlass,
				ApprovalGroupID:    &groupID,
				ApprovalStep:       1,
				ApprovalStepsTotal: 2,
				Status:             domain.StatusApproved,
				DecidedBy:          strPtr("admin-1"),
			},
		},
	}
	svc := NewUsecases(repo).WithIntentPort(&stubIntentPort{})

	err := svc.Approve(context.Background(), uuid.New(), uuid.New(), "admin-1")
	if err == nil {
		t.Fatal("expected duplicate approver rejection")
	}
}

func TestBreakGlassApproveNeedsQuorumBeforeIntentApproved(t *testing.T) {
	intentID := uuid.New()
	groupID := uuid.New()
	repo := &stubRepo{
		intentID: &intentID,
		getItem: &domain.PendingApproval{
			IntentID:           &intentID,
			ApprovalMode:       domain.ApprovalModeBreakGlass,
			ApprovalGroupID:    &groupID,
			ApprovalStep:       1,
			ApprovalStepsTotal: 2,
			Status:             domain.StatusPending,
		},
		byIntent: []domain.PendingApproval{
			{
				ID:                 uuid.New(),
				IntentID:           &intentID,
				ApprovalMode:       domain.ApprovalModeBreakGlass,
				ApprovalGroupID:    &groupID,
				ApprovalStep:       1,
				ApprovalStepsTotal: 2,
				Status:             domain.StatusPending,
			},
		},
	}
	intentPort := &stubIntentPort{}
	svc := NewUsecases(repo).WithIntentPort(intentPort)

	if err := svc.Approve(context.Background(), uuid.New(), uuid.New(), "admin-2"); err != nil {
		t.Fatal(err)
	}
	if intentPort.approved != uuid.Nil {
		t.Fatal("did not expect intent approved before quorum")
	}
}

func TestBreakGlassApproveMarksIntentApprovedAtQuorum(t *testing.T) {
	intentID := uuid.New()
	groupID := uuid.New()
	repo := &stubRepo{
		intentID: &intentID,
		getItem: &domain.PendingApproval{
			IntentID:           &intentID,
			ApprovalMode:       domain.ApprovalModeBreakGlass,
			ApprovalGroupID:    &groupID,
			ApprovalStep:       2,
			ApprovalStepsTotal: 2,
			Status:             domain.StatusPending,
		},
		byIntent: []domain.PendingApproval{
			{
				ID:                 uuid.New(),
				IntentID:           &intentID,
				ApprovalMode:       domain.ApprovalModeBreakGlass,
				ApprovalGroupID:    &groupID,
				ApprovalStep:       1,
				ApprovalStepsTotal: 2,
				Status:             domain.StatusApproved,
				DecidedBy:          strPtr("admin-1"),
			},
		},
	}
	intentPort := &stubIntentPort{}
	svc := NewUsecases(repo).WithIntentPort(intentPort)

	if err := svc.Approve(context.Background(), uuid.New(), uuid.New(), "admin-2"); err != nil {
		t.Fatal(err)
	}
	if intentPort.approved != intentID {
		t.Fatalf("expected approved intent %s, got %s", intentID, intentPort.approved)
	}
}

func TestBreakGlassRejectRejectsWholeIntent(t *testing.T) {
	intentID := uuid.New()
	repo := &stubRepo{
		intentID: &intentID,
		getItem: &domain.PendingApproval{
			IntentID:           &intentID,
			ApprovalMode:       domain.ApprovalModeBreakGlass,
			ApprovalStep:       1,
			ApprovalStepsTotal: 2,
			Status:             domain.StatusPending,
		},
	}
	intentPort := &stubIntentPort{}
	svc := NewUsecases(repo).WithIntentPort(intentPort)

	if err := svc.Reject(context.Background(), uuid.New(), uuid.New(), "admin-1"); err != nil {
		t.Fatal(err)
	}
	if intentPort.rejected != intentID {
		t.Fatalf("expected rejected intent %s, got %s", intentID, intentPort.rejected)
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
