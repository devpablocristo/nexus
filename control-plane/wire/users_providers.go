package wire

import (
	"github.com/google/wire"

	"control-plane/internal/users"
)

var UsersSet = wire.NewSet(
	users.NewRepository,
	users.NewUsecases,
	users.NewHandler,
)
