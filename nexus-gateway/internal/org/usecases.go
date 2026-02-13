package org

import (
	"context"

	"nexus-gateway/internal/org/usecases/domain"
)

type APIKeyRepositoryPort interface {
	FindPrincipalByAPIKeyHash(ctx context.Context, apiKeyHash string) (principal domain.Principal, storedHash string, err error)
}

type AuthUsecase interface {
	ResolvePrincipal(ctx context.Context, apiKeyHash string) (domain.Principal, error)
}

type authService struct {
	repo APIKeyRepositoryPort
}

func NewAuthUsecase(repo APIKeyRepositoryPort) AuthUsecase {
	return &authService{repo: repo}
}

func (s *authService) ResolvePrincipal(ctx context.Context, apiKeyHash string) (domain.Principal, error) {
	principal, _, err := s.repo.FindPrincipalByAPIKeyHash(ctx, apiKeyHash)
	return principal, err
}
