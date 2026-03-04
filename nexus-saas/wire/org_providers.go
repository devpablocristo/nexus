package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/org"
)

var OrgSet = wire.NewSet(
	org.NewRepository,
	wire.Bind(new(org.APIKeyRepositoryPort), new(*org.Repository)),
	org.NewUsecases,
	org.NewHandler,
)
