package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/tool"
)

var ToolSet = wire.NewSet(
	tool.NewRepository,
	wire.Bind(new(tool.RepositoryPort), new(*tool.Repository)),
	tool.NewService,
	wire.Bind(new(tool.Service), new(*tool.ServiceImpl)),
	tool.NewHandler,
)
