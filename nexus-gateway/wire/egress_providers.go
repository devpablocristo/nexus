package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/egress"
	"nexus-gateway/internal/tool"
)

func ProvideEgressToolLookup(s tool.Service) egress.ToolLookupPort { return s }

var EgressSet = wire.NewSet(
	egress.NewRepository,
	wire.Bind(new(egress.RepositoryPort), new(*egress.Repository)),
	ProvideEgressToolLookup,
	egress.NewService,
	egress.NewHandler,
)
