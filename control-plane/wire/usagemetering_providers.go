package wire

import (
	"github.com/google/wire"

	"control-plane/internal/usagemetering"
)

var UsageMeteringSet = wire.NewSet(
	usagemetering.NewRepository,
	wire.Bind(new(usagemetering.MeteringPort), new(*usagemetering.Repository)),
	usagemetering.NewAPICallsMiddleware,
)
