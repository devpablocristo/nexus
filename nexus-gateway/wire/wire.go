//go:build wireinject

package wire

import (
	"github.com/google/wire"

	"nexus-gateway/cmd/config"
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

		ExecutorSet,
		OrgSet,
		ToolSet,
		PolicySet,
		AuditSet,
		GatewaySet,
		MiddlewareSet,

		NewRouter,
		NewHTTPServer,
		NewApp,
	)
	return nil, nil, nil
}
