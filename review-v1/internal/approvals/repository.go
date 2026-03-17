package approvals

import (
	"context"

	"github.com/google/uuid"
	approvaldomain "github.com/devpablocristo/nexus/review-v1/internal/approvals/usecases/domain"
)

type Repository interface {
	Create(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error)
	GetByID(ctx context.Context, id uuid.UUID) (approvaldomain.Approval, error)
	GetByRequestID(ctx context.Context, requestID uuid.UUID) (*approvaldomain.Approval, error)
	ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error)
	Update(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error)
}
