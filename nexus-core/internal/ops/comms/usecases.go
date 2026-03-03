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

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) Create(ctx context.Context, in commsdomain.Draft) (commsdomain.Draft, error) {
	return u.repo.Create(ctx, in)
}

func (u *Usecases) MarkStatus(ctx context.Context, orgID, draftID uuid.UUID, status commsdomain.Status) (commsdomain.Draft, error) {
	return u.repo.MarkStatus(ctx, orgID, draftID, status)
}

func (u *Usecases) ListByIncident(ctx context.Context, orgID, incidentID uuid.UUID, limit int) ([]commsdomain.Draft, error) {
	return u.repo.ListByIncident(ctx, orgID, incidentID, limit)
}
