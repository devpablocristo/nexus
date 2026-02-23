package diagnosis

import (
	"context"

	"github.com/google/uuid"
	diagnosisdomain "nexus-core/internal/ops/diagnosis/usecases/domain"
)

type RepositoryPort interface {
	Create(ctx context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error)
	ListByIncident(ctx context.Context, orgID uuid.UUID, incidentID uuid.UUID, limit int) ([]diagnosisdomain.Report, error)
}

type Service interface {
	Create(ctx context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error)
	ListByIncident(ctx context.Context, orgID uuid.UUID, incidentID uuid.UUID, limit int) ([]diagnosisdomain.Report, error)
}

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error) {
	return s.repo.Create(ctx, in)
}

func (s *service) ListByIncident(ctx context.Context, orgID uuid.UUID, incidentID uuid.UUID, limit int) ([]diagnosisdomain.Report, error) {
	return s.repo.ListByIncident(ctx, orgID, incidentID, limit)
}
