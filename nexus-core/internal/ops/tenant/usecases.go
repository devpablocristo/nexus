package tenant

import (
	"context"

	"github.com/google/uuid"
	tenantdomain "nexus-core/internal/ops/tenant/usecases/domain"
)

type RepositoryPort interface {
	UpsertProfile(ctx context.Context, in tenantdomain.TenantProfile) error
	GetProfile(ctx context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error)
	ListContacts(ctx context.Context, orgID uuid.UUID) ([]tenantdomain.Contact, error)
	UpsertIncidentSettings(ctx context.Context, in tenantdomain.IncidentSettings) error
	GetIncidentSettings(ctx context.Context, orgID uuid.UUID) (tenantdomain.IncidentSettings, error)
}

type Service interface {
	UpsertProfile(ctx context.Context, in tenantdomain.TenantProfile) error
	GetProfile(ctx context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error)
	ListContacts(ctx context.Context, orgID uuid.UUID) ([]tenantdomain.Contact, error)
	UpsertIncidentSettings(ctx context.Context, in tenantdomain.IncidentSettings) error
	GetIncidentSettings(ctx context.Context, orgID uuid.UUID) (tenantdomain.IncidentSettings, error)
}

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) Service {
	return &service{repo: repo}
}

func (s *service) UpsertProfile(ctx context.Context, in tenantdomain.TenantProfile) error {
	return s.repo.UpsertProfile(ctx, in)
}

func (s *service) GetProfile(ctx context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error) {
	return s.repo.GetProfile(ctx, orgID)
}

func (s *service) ListContacts(ctx context.Context, orgID uuid.UUID) ([]tenantdomain.Contact, error) {
	return s.repo.ListContacts(ctx, orgID)
}

func (s *service) UpsertIncidentSettings(ctx context.Context, in tenantdomain.IncidentSettings) error {
	return s.repo.UpsertIncidentSettings(ctx, in)
}

func (s *service) GetIncidentSettings(ctx context.Context, orgID uuid.UUID) (tenantdomain.IncidentSettings, error) {
	return s.repo.GetIncidentSettings(ctx, orgID)
}
