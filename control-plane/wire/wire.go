//go:build wireinject

package wire

import (
	"github.com/google/wire"

	"control-plane/cmd/config"
)

func InitializeAPI(cfg config.Config) (*App, func(), error) {
	wire.Build(
		ProvideAPIConfig,
		ProvideDBConfig,
		ProvideHTTPServerConfig,
		ProvideServiceConfig,

		NewLogger,
		NewGormConfig,
		NewDB,

		IdentitySet,
		OrgSet,
		UsersSet,
		AdminSet,
		NotificationsSet,
		BillingSet,
		EventsSet,
		ActionsSet,
		IncidentsSet,
		PolicyProposalSet,
		AssistantSet,
		AlertsSet,
		SessionSet,
		MiddlewareSet,
		UsageMeteringSet,
		ContractsSet,
		CoreProxySet,
		ClerkWebhookSet,

		NewRouter,
		NewHTTPServer,
		NewApp,
	)
	return nil, nil, nil
}
