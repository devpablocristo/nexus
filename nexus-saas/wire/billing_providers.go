package wire

import (
	"github.com/google/wire"

	"nexus-saas/cmd/config"
	"nexus-saas/internal/admin"
	"nexus-saas/internal/billing"
)

func ProvideStripeClient(cfg config.ServiceConfig) *billing.StripeClient {
	return billing.NewStripeClient(cfg.StripeSecretKey)
}

func ProvideTenantSettingsPort(repo *admin.Repository) billing.TenantSettingsPort {
	return repo
}

var BillingSet = wire.NewSet(
	billing.NewRepository,
	ProvideStripeClient,
	ProvideTenantSettingsPort,
	billing.NewUsecases,
	billing.NewHandler,
)
