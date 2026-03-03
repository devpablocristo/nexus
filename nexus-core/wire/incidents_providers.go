package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/events"
	"nexus-core/internal/incidents"
)

func ProvideIncidentsEventSink(s *events.Usecases) incidents.EventSink { return s }

var IncidentsSet = wire.NewSet(
	incidents.NewRepository,
	wire.Bind(new(incidents.RepositoryPort), new(*incidents.Repository)),
	ProvideIncidentsEventSink,
	incidents.NewUsecases,
	incidents.NewHandler,
)
