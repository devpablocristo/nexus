package wire

import (
	"github.com/google/wire"

	"data-plane/internal/audit"
)

var AuditSet = wire.NewSet(
	audit.NewRepository,
	wire.Bind(new(audit.RepositoryPort), new(*audit.Repository)),
	audit.NewUsecases,
	audit.AsAuditUsecase,
	audit.NewHandler,
)
