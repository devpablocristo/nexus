package audit

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	auditdomain "nexus-gateway/internal/audit/usecases/domain"
	"nexus-gateway/pkg/types"
)

type RepositoryPort interface {
	Create(ctx context.Context, ev auditdomain.AuditEvent) error
	Query(ctx context.Context, orgID uuid.UUID, q auditdomain.Query) ([]auditdomain.AuditEvent, error)
}

type Service interface {
	Query(ctx context.Context, orgID uuid.UUID, q auditdomain.Query) ([]auditdomain.AuditEvent, error)
}

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) Service {
	return &service{repo: repo}
}

func (s *service) Query(ctx context.Context, orgID uuid.UUID, q auditdomain.Query) ([]auditdomain.AuditEvent, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 200 {
		return nil, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "limit must be <= 200")
	}
	return s.repo.Query(ctx, orgID, q)
}
