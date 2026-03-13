package wire

import (
	"github.com/google/wire"

	"data-plane/internal/actionoverrides"
	"data-plane/internal/gateway"
)

var ActionOverridesSet = wire.NewSet(
	actionoverrides.NewRepository,
	wire.Bind(new(gateway.ActionOverridesPort), new(*actionoverrides.Repository)),
)
