package wire

import (
	"github.com/google/wire"

	"data-plane/internal/admin"
	"data-plane/internal/tool"
)

func ProvideToolTenantLimits(r *admin.Repository) tool.TenantLimitsPort { return r }

func ProvideToolHandler(uc *tool.Usecases) *tool.Handler { return tool.NewHandler(uc) }

var ToolSet = wire.NewSet(
	tool.NewRepository,
	wire.Bind(new(tool.RepositoryPort), new(*tool.Repository)),
	ProvideToolTenantLimits,
	tool.NewUsecases,
	ProvideToolHandler,
)
