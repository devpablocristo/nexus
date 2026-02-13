package usecases

import (
	"context"

	"github.com/google/uuid"
)

type APIKeyRepositoryPort interface {
	FindOrgIDByAPIKeyHash(ctx context.Context, apiKeyHash string) (orgID uuid.UUID, storedHash string, err error)
}

type AuthUsecase interface {
	ResolveOrgID(ctx context.Context, apiKeyHash string) (uuid.UUID, error)
}

type authService struct {
	repo APIKeyRepositoryPort
}

func NewAuthUsecase(repo APIKeyRepositoryPort) AuthUsecase {
	return &authService{repo: repo}
}

func (s *authService) ResolveOrgID(ctx context.Context, apiKeyHash string) (uuid.UUID, error) {
	orgID, _, err := s.repo.FindOrgIDByAPIKeyHash(ctx, apiKeyHash)
	return orgID, err
}
