package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/admin"
	"nexus-core/internal/tool"
)

func ProvideToolTenantLimits(r *admin.Repository) tool.TenantLimitsPort { return r }

var ToolSet = wire.NewSet(
	tool.NewRepository,
	wire.Bind(new(tool.RepositoryPort), new(*tool.Repository)),
	ProvideToolTenantLimits,
	tool.NewService,
	wire.Bind(new(tool.Service), new(*tool.ServiceImpl)),
	tool.NewHandler,
)
