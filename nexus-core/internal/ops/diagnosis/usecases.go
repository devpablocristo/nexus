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

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) Create(ctx context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error) {
	return u.repo.Create(ctx, in)
}

func (u *Usecases) ListByIncident(ctx context.Context, orgID uuid.UUID, incidentID uuid.UUID, limit int) ([]diagnosisdomain.Report, error) {
	return u.repo.ListByIncident(ctx, orgID, incidentID, limit)
}
