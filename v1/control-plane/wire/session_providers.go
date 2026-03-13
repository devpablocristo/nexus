package wire

import (
	"github.com/google/wire"

	"control-plane/internal/session"
)

var SessionSet = wire.NewSet(
	session.NewRepository,
	wire.Bind(new(session.RepoPort), new(*session.Repository)),
	session.NewUsecases,
	session.NewHandler,
)
