package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/admin"
)

var AdminSet = wire.NewSet(
	admin.NewRepository,
	wire.Bind(new(admin.RepositoryPort), new(*admin.Repository)),
	admin.NewService,
	admin.NewHandler,
)
