package audit

import (
	"context"

	"github.com/google/uuid"
	auditdomain "github.com/devpablocristo/nexus/review-v1/internal/audit/usecases/domain"
)

type Repository interface {
	Append(ctx context.Context, e auditdomain.RequestEvent) error
	ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]auditdomain.RequestEvent, error)
}
