package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/audit"
)

var AuditSet = wire.NewSet(
	audit.NewRepository,
	wire.Bind(new(audit.RepositoryPort), new(*audit.Repository)),
	audit.NewService,
	audit.NewHandler,
)
