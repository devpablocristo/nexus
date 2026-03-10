package wire

import (
	"github.com/google/wire"

	"data-plane/internal/admin"
)

var AdminSet = wire.NewSet(
	admin.NewRepository,
)
