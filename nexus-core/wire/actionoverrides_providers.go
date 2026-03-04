package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/actionoverrides"
	"nexus-core/internal/gateway"
)

var ActionOverridesSet = wire.NewSet(
	actionoverrides.NewRepository,
	wire.Bind(new(gateway.ActionOverridesPort), new(*actionoverrides.Repository)),
)
