package egress

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus-core/pkg/types"
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

type Usecases struct {
	repo RepositoryPort
	tool ToolLookupPort
}

func NewUsecases(repo RepositoryPort, tool ToolLookupPort) *Usecases {
	return &Usecases{repo: repo, tool: tool}
}

func (u *Usecases) UpsertRule(ctx context.Context, orgID uuid.UUID, toolName, host string, enabled bool) error {
	t, err := u.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return err
	}
	host = normalizeHost(host)
	if host == "" {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "host required")
	}
	return u.repo.Upsert(ctx, orgID, t.ID, host, enabled)
}

func (u *Usecases) ListRules(ctx context.Context, orgID uuid.UUID, toolName string) ([]string, error) {
	t, err := u.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return nil, err
	}
	return u.repo.List(ctx, orgID, t.ID)
}

func (u *Usecases) DeleteRule(ctx context.Context, orgID uuid.UUID, toolName, host string) error {
	t, err := u.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return err
	}
	return u.repo.Delete(ctx, orgID, t.ID, normalizeHost(host))
}

func (u *Usecases) IsHostAllowed(ctx context.Context, orgID, toolID uuid.UUID, host string) (bool, error) {
	host = normalizeHost(host)
	hasAny, err := u.repo.HasAny(ctx, orgID, toolID)
	if err != nil {
		return false, err
	}
	if !hasAny {
		return false, nil // default-deny: no rules means no egress allowed
	}
	return u.repo.ExistsHost(ctx, orgID, toolID, host)
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}
