package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/alerts"
)

var AlertsSet = wire.NewSet(
	alerts.NewRepository,
	wire.Bind(new(alerts.RepoPort), new(*alerts.Repository)),
	alerts.NewAuditMetricsSource,
	wire.Bind(new(alerts.MetricsSource), new(*alerts.AuditMetricsSource)),
	alerts.NewUsecases,
	alerts.NewHandler,
)
