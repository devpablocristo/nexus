package wire

import (
	"github.com/google/wire"

	toolhandler "nexus-gateway/internal/tool/handler"
	toolrepo "nexus-gateway/internal/tool/repository"
	tooluc "nexus-gateway/internal/tool/usecases"
)

var ToolSet = wire.NewSet(
	toolrepo.NewRepository,
	wire.Bind(new(tooluc.RepositoryPort), new(*toolrepo.Repository)),
	tooluc.NewService,
	wire.Bind(new(tooluc.Service), new(*tooluc.ServiceImpl)),
	toolhandler.NewHandler,
)
