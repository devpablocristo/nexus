package policies

import (
	"context"
	"errors"

	"github.com/google/uuid"
	policydomain "github.com/devpablocristo/nexus/review-v1/internal/policies/usecases/domain"
)

// Sentinel errors
var (
	ErrNotFound      = errors.New("policy not found")
	ErrAlreadyExists = errors.New("policy already exists")
	ErrArchived      = errors.New("policy is archived")
)

// ListFilters define los filtros para listar políticas.
type ListFilters struct {
	IncludeArchived bool
	EnabledOnly     bool
}

type Repository interface {
	Create(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error)
	GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error)
	Update(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) error
	RestoreByID(ctx context.Context, id uuid.UUID) error
}
