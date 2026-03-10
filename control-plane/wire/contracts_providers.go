package wire

import (
	"github.com/google/wire"

	"control-plane/internal/contracts"
)

var ContractsSet = wire.NewSet(
	contracts.NewHandler,
)

