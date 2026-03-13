package wire

import (
	"github.com/google/wire"

	"data-plane/internal/approval"
	"data-plane/internal/audit"
	"data-plane/internal/gateway"
)

func ProvideGatewayApprovalPort(a *approval.GatewayAdapter) gateway.ApprovalPort {
	return a
}

func ProvideApprovalIntentStatusPort(r *gateway.IntentRepository) approval.IntentStatusPort {
	return r
}

func ProvideApprovalUsecases(repo *approval.Repository, intentPort approval.IntentStatusPort, auditRepo *audit.Repository) *approval.Usecases {
	return approval.NewUsecases(repo).WithIntentPort(intentPort).WithAuditPort(auditRepo)
}

var ApprovalSet = wire.NewSet(
	approval.NewRepository,
	wire.Bind(new(approval.RepoPort), new(*approval.Repository)),
	ProvideApprovalIntentStatusPort,
	ProvideApprovalUsecases,
	approval.NewGatewayAdapter,
	ProvideGatewayApprovalPort,
	approval.NewHandler,
)
