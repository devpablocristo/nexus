package org

import (
	"context"

	"nexus-saas/internal/org/usecases/domain"
)

type APIKeyRepositoryPort interface {
	FindPrincipalByAPIKeyHash(ctx context.Context, apiKeyHash string) (principal domain.Principal, storedHash string, err error)
}

type Usecases struct {
	repo APIKeyRepositoryPort
}

func NewUsecases(repo APIKeyRepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) ResolvePrincipal(ctx context.Context, apiKeyHash string) (domain.Principal, error) {
	principal, _, err := u.repo.FindPrincipalByAPIKeyHash(ctx, apiKeyHash)
	return principal, err
}
