package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/events"
)

var EventsSet = wire.NewSet(
	events.NewRepository,
	wire.Bind(new(events.RepositoryPort), new(*events.Repository)),
	events.NewUsecases,
	events.NewHandler,
)
