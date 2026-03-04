package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/approval"
	"nexus-core/internal/gateway"
)

func ProvideGatewayApprovalPort(a *approval.GatewayAdapter) gateway.ApprovalPort {
	return a
}

var ApprovalSet = wire.NewSet(
	approval.NewRepository,
	wire.Bind(new(approval.RepoPort), new(*approval.Repository)),
	approval.NewUsecases,
	approval.NewGatewayAdapter,
	ProvideGatewayApprovalPort,
	approval.NewHandler,
)
