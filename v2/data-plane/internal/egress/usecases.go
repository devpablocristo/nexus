package egress

import (
	"context"
	"strings"
)

type RepositoryPort interface {
	HasAny(ctx context.Context, toolID string) (bool, error)
	ExistsHost(ctx context.Context, toolID, host string) (bool, error)
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) IsHostAllowed(ctx context.Context, toolID, host string) (bool, error) {
	host = normalizeHost(host)
	hasAny, err := u.repo.HasAny(ctx, toolID)
	if err != nil {
		return false, err
	}
	if !hasAny {
		return false, nil
	}
	return u.repo.ExistsHost(ctx, toolID, host)
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}
