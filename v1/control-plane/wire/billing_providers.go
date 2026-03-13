package wire

import (
	"github.com/google/wire"

	"control-plane/cmd/config"
	"control-plane/internal/admin"
	"control-plane/internal/billing"
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
	ProvideBillingNotificationPort,
	billing.NewUsecases,
	billing.NewHandler,
)
