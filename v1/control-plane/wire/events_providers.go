package wire

import (
	"github.com/google/wire"

	"control-plane/internal/events"
	"control-plane/internal/usagemetering"
)

func ProvideEventsMeteringPort(r *usagemetering.Repository) events.MeteringPort { return r }

var EventsSet = wire.NewSet(
	events.NewRepository,
	wire.Bind(new(events.RepositoryPort), new(*events.Repository)),
	ProvideEventsMeteringPort,
	events.NewUsecases,
	events.NewHandler,
)
