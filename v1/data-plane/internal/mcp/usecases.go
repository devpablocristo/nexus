package mcp

import (
	"context"

	"github.com/google/uuid"

	gwdomain "data-plane/internal/gateway/usecases/domain"
	tooldomain "data-plane/internal/tool/usecases/domain"
)

type ToolPort interface {
	List(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error)
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
}

type RunPort interface {
	Run(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
}

type Usecases struct {
	tool ToolPort
	run  RunPort
}

func NewUsecases(tool ToolPort, run RunPort) *Usecases {
	return &Usecases{tool: tool, run: run}
}

func (u *Usecases) ListTools(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error) {
	return u.tool.List(ctx, orgID)
}

func (u *Usecases) GetTool(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error) {
	return u.tool.GetByName(ctx, orgID, name)
}

func (u *Usecases) CallTool(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	return u.run.Run(ctx, orgID, req)
}
