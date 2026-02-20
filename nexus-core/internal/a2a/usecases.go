package a2a

import (
	"context"

	"github.com/google/uuid"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
)

type RunPort interface {
	Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type Service interface {
	CallTool(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type service struct {
	run RunPort
}

func NewService(run RunPort) Service {
	return &service{run: run}
}

func (s *service) CallTool(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	return s.run.Run(ctx, orgID, req)
}
