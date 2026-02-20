package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/a2a"
	"nexus-core/internal/gateway"
)

func ProvideA2ARunPort(s gateway.Service) a2a.RunPort { return s }

var A2ASet = wire.NewSet(
	ProvideA2ARunPort,
	a2a.NewService,
	a2a.NewHandler,
)
