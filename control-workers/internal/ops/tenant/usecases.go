package tenant

import (
	"context"

	"github.com/google/uuid"
	tenantdomain "control-workers/internal/ops/tenant/usecases/domain"
)

type RepositoryPort interface {
	UpsertProfile(ctx context.Context, in tenantdomain.TenantProfile) error
	GetProfile(ctx context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error)
	ListContacts(ctx context.Context, orgID uuid.UUID) ([]tenantdomain.Contact, error)
	UpsertIncidentSettings(ctx context.Context, in tenantdomain.IncidentSettings) error
	GetIncidentSettings(ctx context.Context, orgID uuid.UUID) (tenantdomain.IncidentSettings, error)
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) UpsertProfile(ctx context.Context, in tenantdomain.TenantProfile) error {
	return u.repo.UpsertProfile(ctx, in)
}

func (u *Usecases) GetProfile(ctx context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error) {
	return u.repo.GetProfile(ctx, orgID)
}

func (u *Usecases) ListContacts(ctx context.Context, orgID uuid.UUID) ([]tenantdomain.Contact, error) {
	return u.repo.ListContacts(ctx, orgID)
}

func (u *Usecases) UpsertIncidentSettings(ctx context.Context, in tenantdomain.IncidentSettings) error {
	return u.repo.UpsertIncidentSettings(ctx, in)
}

func (u *Usecases) GetIncidentSettings(ctx context.Context, orgID uuid.UUID) (tenantdomain.IncidentSettings, error) {
	return u.repo.GetIncidentSettings(ctx, orgID)
}
