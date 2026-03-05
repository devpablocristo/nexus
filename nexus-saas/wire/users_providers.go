package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/users"
)

var UsersSet = wire.NewSet(
	users.NewRepository,
	users.NewUsecases,
	users.NewHandler,
)
