package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/coreproxy"
)

var CoreProxySet = wire.NewSet(
	coreproxy.NewHandler,
)
