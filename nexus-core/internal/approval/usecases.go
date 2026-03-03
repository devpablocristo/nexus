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

type Service struct {
	repo RepoPort
}

func NewService(repo RepoPort) *Service {
	return &Service{repo: repo}
}

func (s *Service) RequestApproval(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	if req.TTLSeconds <= 0 {
		req.TTLSeconds = 3600
	}
	return s.repo.Create(ctx, req)
}

func (s *Service) ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error) {
	return s.repo.ListPending(ctx, orgID, limit)
}

func (s *Service) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error) {
	return s.repo.GetByID(ctx, orgID, id)
}

func (s *Service) Approve(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	return s.repo.Decide(ctx, orgID, id, domain.StatusApproved, decidedBy)
}

func (s *Service) Reject(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	return s.repo.Decide(ctx, orgID, id, domain.StatusRejected, decidedBy)
}

func (s *Service) ExpireOld(ctx context.Context) (int64, error) {
	return s.repo.ExpireOld(ctx)
}
