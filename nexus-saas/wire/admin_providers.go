package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/admin"
)

var AdminSet = wire.NewSet(
	admin.NewRepository,
	wire.Bind(new(admin.RepositoryPort), new(*admin.Repository)),
	admin.NewUsecases,
	admin.NewHandler,
)
