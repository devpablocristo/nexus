package wire

import (
	"github.com/google/wire"

	"data-plane/internal/egress"
	"data-plane/internal/tool"
)

func ProvideEgressToolLookup(s *tool.Usecases) egress.ToolLookupPort { return s }

var EgressSet = wire.NewSet(
	egress.NewRepository,
	wire.Bind(new(egress.RepositoryPort), new(*egress.Repository)),
	ProvideEgressToolLookup,
	egress.NewUsecases,
	egress.AsEgressUsecase,
	egress.NewHandler,
)
