package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/org"
)

var OrgSet = wire.NewSet(
	org.NewRepository,
	wire.Bind(new(org.APIKeyRepositoryPort), new(*org.Repository)),
	org.NewAuthUsecase,
	org.NewHandler,
)
