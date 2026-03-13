package wire

import (
	"github.com/google/wire"

	"control-plane/internal/events"
	"control-plane/internal/incidents"
	"control-plane/internal/usagemetering"
)

func ProvideIncidentsEventSink(s *events.Usecases) incidents.EventSink                { return s }
func ProvideIncidentsMeteringPort(r *usagemetering.Repository) incidents.MeteringPort { return r }

var IncidentsSet = wire.NewSet(
	incidents.NewRepository,
	wire.Bind(new(incidents.RepositoryPort), new(*incidents.Repository)),
	ProvideIncidentsEventSink,
	ProvideIncidentsMeteringPort,
	ProvideIncidentsNotificationPort,
	incidents.NewUsecases,
	incidents.NewHandler,
)
