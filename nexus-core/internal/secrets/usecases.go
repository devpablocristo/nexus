package secrets

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	secretdomain "nexus-core/internal/secrets/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus-core/pkg/types"
)

type RepositoryPort interface {
	UpsertForTool(ctx context.Context, orgID, toolID uuid.UUID, secret secretdomain.ToolSecret) (secretdomain.ToolSecret, error)
	ListForTool(ctx context.Context, orgID, toolID uuid.UUID) ([]secretdomain.ToolSecret, error)
	DeleteForTool(ctx context.Context, orgID, toolID uuid.UUID, keyName string) error
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

func (u *Usecases) UpsertForTool(ctx context.Context, orgID uuid.UUID, toolName, secretType, keyName, value string, enabled bool) (secretdomain.ToolSecret, error) {
	t, err := u.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return secretdomain.ToolSecret{}, err
	}
	secretType = strings.ToLower(strings.TrimSpace(secretType))
	if secretType != "header" && secretType != "bearer" {
		return secretdomain.ToolSecret{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "secret_type must be header|bearer")
	}
	if secretType == "header" && strings.TrimSpace(keyName) == "" {
		return secretdomain.ToolSecret{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "key_name required for header")
	}
	if strings.TrimSpace(value) == "" {
		return secretdomain.ToolSecret{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "value required")
	}
	return u.repo.UpsertForTool(ctx, orgID, t.ID, secretdomain.ToolSecret{
		OrgID:          orgID,
		ToolID:         t.ID,
		SecretType:     secretType,
		KeyName:        strings.TrimSpace(keyName),
		PlaintextValue: value,
		Enabled:        enabled,
	})
}

func (u *Usecases) ListForTool(ctx context.Context, orgID uuid.UUID, toolName string) ([]secretdomain.ToolSecret, error) {
	t, err := u.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return nil, err
	}
	return u.repo.ListForTool(ctx, orgID, t.ID)
}

func (u *Usecases) DeleteForTool(ctx context.Context, orgID uuid.UUID, toolName, keyName string) error {
	t, err := u.tool.GetByName(ctx, orgID, toolName)
	if err != nil {
		return err
	}
	return u.repo.DeleteForTool(ctx, orgID, t.ID, keyName)
}
