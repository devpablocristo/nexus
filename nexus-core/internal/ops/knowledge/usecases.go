package knowledge

import (
	"context"

	"github.com/google/uuid"
	knowledgedomain "nexus-core/internal/ops/knowledge/usecases/domain"
)

type RepositoryPort interface {
	Create(ctx context.Context, in knowledgedomain.Document) (knowledgedomain.Document, error)
	SearchDeterministic(ctx context.Context, orgID uuid.UUID, query string, docType *knowledgedomain.DocType, limit int) ([]knowledgedomain.Document, error)
}

type Service interface {
	Create(ctx context.Context, in knowledgedomain.Document) (knowledgedomain.Document, error)
	SearchDeterministic(ctx context.Context, orgID uuid.UUID, query string, docType *knowledgedomain.DocType, limit int) ([]knowledgedomain.Document, error)
}

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, in knowledgedomain.Document) (knowledgedomain.Document, error) {
	return s.repo.Create(ctx, in)
}

func (s *service) SearchDeterministic(ctx context.Context, orgID uuid.UUID, query string, docType *knowledgedomain.DocType, limit int) ([]knowledgedomain.Document, error) {
	return s.repo.SearchDeterministic(ctx, orgID, query, docType, limit)
}
