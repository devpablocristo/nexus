package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/approval"
	"nexus-core/internal/gateway"
)

func ProvideGatewayApprovalPort(a *approval.GatewayAdapter) gateway.ApprovalPort {
	return a
}

func ProvideApprovalIntentStatusPort(r *gateway.IntentRepository) approval.IntentStatusPort {
	return r
}

func ProvideApprovalUsecases(repo *approval.Repository, intentPort approval.IntentStatusPort) *approval.Usecases {
	return approval.NewUsecases(repo).WithIntentPort(intentPort)
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
