package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/admin"
)

var AdminSet = wire.NewSet(
	admin.NewRepository,
)
