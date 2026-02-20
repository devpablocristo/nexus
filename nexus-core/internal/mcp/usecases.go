package mcp

import (
	"context"

	"github.com/google/uuid"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
)

type ToolPort interface {
	List(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error)
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
}

type RunPort interface {
	Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type Service interface {
	ListTools(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error)
	GetTool(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
	CallTool(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type service struct {
	tool ToolPort
	run  RunPort
}

func NewService(tool ToolPort, run RunPort) Service {
	return &service{tool: tool, run: run}
}

func (s *service) ListTools(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error) {
	return s.tool.List(ctx, orgID)
}

func (s *service) GetTool(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error) {
	return s.tool.GetByName(ctx, orgID, name)
}

func (s *service) CallTool(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	return s.run.Run(ctx, orgID, req)
}
