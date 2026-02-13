package egress

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	tooldomain "nexus-gateway/internal/tool/usecases/domain"
	"nexus-gateway/pkg/types"
)

type RepositoryPort interface {
	Upsert(ctx context.Context, orgID, toolID uuid.UUID, host string, enabled bool) error
	List(ctx context.Context, orgID, toolID uuid.UUID) ([]string, error)
	Delete(ctx context.Context, orgID, toolID uuid.UUID, host string) error
	HasAny(ctx context.Context, orgID, toolID uuid.UUID) (bool, error)
	ExistsHost(ctx context.Context, orgID, toolID uuid.UUID, host string) (bool, error)
}

type ToolLookupPort interface {
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
}

type Service interface {
	UpsertRule(ctx context.Context, orgID uuid.UUID, toolName, host string, enabled bool) error
	ListRules(ctx context.Context, orgID uuid.UUID, toolName string) ([]string, error)
	DeleteRule(ctx context.Context, orgID uuid.UUID, toolName, host string) error
	IsHostAllowed(ctx context.Context, orgID, toolID uuid.UUID, host string) (bool, error)
}

type service struct {
	repo RepositoryPort
	tool ToolLookupPort
}

func NewService(repo RepositoryPort, tool ToolLookupPort) Service {
	return &service{repo: repo, tool: tool}
}

func (s *service) UpsertRule(ctx context.Context, orgID uuid.UUID, toolName, host string, enabled bool) error {
	t, err := s.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return err
	}
	host = normalizeHost(host)
	if host == "" {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "host required")
	}
	return s.repo.Upsert(ctx, orgID, t.ID, host, enabled)
}

func (s *service) ListRules(ctx context.Context, orgID uuid.UUID, toolName string) ([]string, error) {
	t, err := s.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, orgID, t.ID)
}

func (s *service) DeleteRule(ctx context.Context, orgID uuid.UUID, toolName, host string) error {
	t, err := s.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, orgID, t.ID, normalizeHost(host))
}

func (s *service) IsHostAllowed(ctx context.Context, orgID, toolID uuid.UUID, host string) (bool, error) {
	host = normalizeHost(host)
	hasAny, err := s.repo.HasAny(ctx, orgID, toolID)
	if err != nil {
		return false, err
	}
	if !hasAny {
		return true, nil
	}
	return s.repo.ExistsHost(ctx, orgID, toolID, host)
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}
