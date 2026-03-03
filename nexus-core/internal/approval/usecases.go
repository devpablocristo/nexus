package approval

import (
	"context"

	"github.com/google/uuid"

	domain "nexus-core/internal/approval/usecases/domain"
)

type RepoPort interface {
	Create(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error)
	ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error)
	Decide(ctx context.Context, orgID, id uuid.UUID, status domain.Status, decidedBy string) error
	ExpireOld(ctx context.Context) (int64, error)
}

type Usecases struct {
	repo RepoPort
}

func NewUsecases(repo RepoPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) RequestApproval(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	if req.TTLSeconds <= 0 {
		req.TTLSeconds = 3600
	}
	return u.repo.Create(ctx, req)
}

func (u *Usecases) ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error) {
	return u.repo.ListPending(ctx, orgID, limit)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error) {
	return u.repo.GetByID(ctx, orgID, id)
}

func (u *Usecases) Approve(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	return u.repo.Decide(ctx, orgID, id, domain.StatusApproved, decidedBy)
}

func (u *Usecases) Reject(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	return u.repo.Decide(ctx, orgID, id, domain.StatusRejected, decidedBy)
}

func (u *Usecases) ExpireOld(ctx context.Context) (int64, error) {
	return u.repo.ExpireOld(ctx)
}
