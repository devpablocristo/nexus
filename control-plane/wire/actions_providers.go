package wire

import (
	"github.com/google/wire"

	"control-plane/internal/actions"
	"control-plane/internal/events"
	"control-plane/internal/usagemetering"
)

func ProvideActionsEventSink(s *events.Usecases) actions.EventSink { return s }
func ProvideActionsMeteringPort(r *usagemetering.Repository) actions.MeteringPort { return r }

var ActionsSet = wire.NewSet(
	actions.NewRepository,
	wire.Bind(new(actions.RepositoryPort), new(*actions.Repository)),
	ProvideActionsEventSink,
	ProvideActionsMeteringPort,
	actions.NewUsecases,
	actions.NewHandler,
)
