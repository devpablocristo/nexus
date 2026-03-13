package wire

import (
	"github.com/google/wire"

	"control-plane/internal/coreproxy"
)

var CoreProxySet = wire.NewSet(
	coreproxy.NewHandler,
)
