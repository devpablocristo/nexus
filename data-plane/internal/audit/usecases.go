package audit

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	auditdomain "data-plane/internal/audit/usecases/domain"
	"nexus/pkg/types"
)

type RepositoryPort interface {
	Create(ctx context.Context, ev auditdomain.AuditEvent) error
	Query(ctx context.Context, orgID uuid.UUID, q auditdomain.Query) ([]auditdomain.AuditEvent, error)
	StreamByFilters(ctx context.Context, orgID uuid.UUID, q auditdomain.Query, batchSize int, fn func(auditdomain.AuditEvent) error) error
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) Query(ctx context.Context, orgID uuid.UUID, q auditdomain.Query) ([]auditdomain.AuditEvent, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 200 {
		return nil, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "limit must be <= 200")
	}
	return u.repo.Query(ctx, orgID, q)
}

func (u *Usecases) StreamByFilters(ctx context.Context, orgID uuid.UUID, q auditdomain.Query, batchSize int, fn func(auditdomain.AuditEvent) error) error {
	if batchSize <= 0 {
		batchSize = 200
	}
	if batchSize > 1000 {
		batchSize = 1000
	}
	q.OrderAsc = true
	return u.repo.StreamByFilters(ctx, orgID, q, batchSize, fn)
}
