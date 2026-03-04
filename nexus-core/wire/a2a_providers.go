package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/a2a"
	"nexus-core/internal/gateway"
)

func ProvideA2ARunPort(s *gateway.Usecases) a2a.RunPort { return s }

var A2ASet = wire.NewSet(
	ProvideA2ARunPort,
	a2a.NewUsecases,
	a2a.AsA2AUsecase,
	a2a.NewHandler,
)
