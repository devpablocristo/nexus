package wire

import (
	"github.com/google/wire"

	policyhandler "nexus-gateway/internal/policy/handler"
	policyrepo "nexus-gateway/internal/policy/repository"
	policyuc "nexus-gateway/internal/policy/usecases"
)

var PolicySet = wire.NewSet(
	policyrepo.NewRepository,
	wire.Bind(new(policyuc.PolicyRepositoryPort), new(*policyrepo.Repository)),
	policyuc.NewEvaluator,
	policyuc.NewService,
	policyhandler.NewHandler,
)
