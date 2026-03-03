package a2a

import (
	"context"

	"github.com/google/uuid"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
)

type RunPort interface {
	Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type Usecases struct {
	run RunPort
}

func NewUsecases(run RunPort) *Usecases {
	return &Usecases{run: run}
}

func (u *Usecases) CallTool(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	return u.run.Run(ctx, orgID, req)
}
