package requests

import (
	"context"

	"github.com/google/uuid"
	requestdomain "github.com/devpablocristo/nexus/review-v1/internal/requests/usecases/domain"
)

type Repository interface {
	Create(ctx context.Context, r requestdomain.Request) (requestdomain.Request, error)
	GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*requestdomain.Request, error)
	List(ctx context.Context, status string, actionType string, limit int) ([]requestdomain.Request, error)
	Update(ctx context.Context, r requestdomain.Request) (requestdomain.Request, error)
}

type IdempotencyStore interface {
	Get(ctx context.Context, key string) (requestID uuid.UUID, response map[string]any, ok bool)
	Set(ctx context.Context, key string, requestID uuid.UUID, response map[string]any, expiresAt interface{}) error
}
