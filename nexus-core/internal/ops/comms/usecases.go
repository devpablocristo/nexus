package comms

import (
	"context"

	"github.com/google/uuid"
	commsdomain "nexus-core/internal/ops/comms/usecases/domain"
)

type RepositoryPort interface {
	Create(ctx context.Context, in commsdomain.Draft) (commsdomain.Draft, error)
	MarkStatus(ctx context.Context, orgID, draftID uuid.UUID, status commsdomain.Status) (commsdomain.Draft, error)
	ListByIncident(ctx context.Context, orgID, incidentID uuid.UUID, limit int) ([]commsdomain.Draft, error)
}

type Service interface {
	Create(ctx context.Context, in commsdomain.Draft) (commsdomain.Draft, error)
	MarkStatus(ctx context.Context, orgID, draftID uuid.UUID, status commsdomain.Status) (commsdomain.Draft, error)
	ListByIncident(ctx context.Context, orgID, incidentID uuid.UUID, limit int) ([]commsdomain.Draft, error)
}

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, in commsdomain.Draft) (commsdomain.Draft, error) {
	return s.repo.Create(ctx, in)
}

func (s *service) MarkStatus(ctx context.Context, orgID, draftID uuid.UUID, status commsdomain.Status) (commsdomain.Draft, error) {
	return s.repo.MarkStatus(ctx, orgID, draftID, status)
}

func (s *service) ListByIncident(ctx context.Context, orgID, incidentID uuid.UUID, limit int) ([]commsdomain.Draft, error) {
	return s.repo.ListByIncident(ctx, orgID, incidentID, limit)
}
