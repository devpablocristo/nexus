//go:build wireinject

package wire

import (
	"github.com/google/wire"

	"data-plane/cmd/config"
)

func InitializeAPI(cfg config.Config) (*App, func(), error) {
	wire.Build(
		ProvideAPIConfig,
		ProvideDBConfig,
		ProvideHTTPServerConfig,
		ProvideServiceConfig,
		ProvideGatewayConfig,

		NewLogger,
		NewGormConfig,
		NewDB,
		NewSchemaCache,
		NewMasterCrypto,
		NewDLPDetector,

		ExecutorSet,
		IdentitySet,
		OrgSet,
		ToolSet,
		PolicySet,
		AuditSet,
		AdminSet,
		ApprovalSet,
		ActionOverridesSet,
		SecretsSet,
		EgressSet,
		GatewaySet,
		MCPSet,
		A2ASet,
		MiddlewareSet,
		UsageMeteringSet,

		NewRouter,
		NewHTTPServer,
		NewApp,
	)
	return nil, nil, nil
}
