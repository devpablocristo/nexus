package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/contracts"
)

var ContractsSet = wire.NewSet(
	contracts.NewHandler,
)

