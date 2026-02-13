package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/org"
)

var OrgSet = wire.NewSet(
	org.NewRepository,
	wire.Bind(new(org.APIKeyRepositoryPort), new(*org.Repository)),
	org.NewAuthUsecase,
)
