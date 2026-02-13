package wire

import (
	"github.com/google/wire"

	audithandler "nexus-gateway/internal/audit/handler"
	auditrepo "nexus-gateway/internal/audit/repository"
	audituc "nexus-gateway/internal/audit/usecases"
)

var AuditSet = wire.NewSet(
	auditrepo.NewRepository,
	wire.Bind(new(audituc.RepositoryPort), new(*auditrepo.Repository)),
	audituc.NewService,
	audithandler.NewHandler,
)
