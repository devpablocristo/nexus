package wire

import (
	"github.com/google/wire"

	orgrepo "nexus-gateway/internal/org/repository"
	orguc "nexus-gateway/internal/org/usecases"
)

var OrgSet = wire.NewSet(
	orgrepo.NewRepository,
	wire.Bind(new(orguc.APIKeyRepositoryPort), new(*orgrepo.Repository)),
	orguc.NewAuthUsecase,
)
