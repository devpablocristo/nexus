package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/admin"
	"nexus-core/internal/audit"
	"nexus-core/internal/egress"
	"nexus-core/internal/gateway"
	exechttp "nexus-core/internal/gateway/executor/http"
	"nexus-core/internal/gateway/executor/ratelimit"
	exectelemetry "nexus-core/internal/gateway/executor/telemetry"
	"nexus-core/internal/policy"
	"nexus-core/internal/secrets"
	"nexus-core/internal/tool"
)

func ProvideGatewayToolRepo(r *tool.Repository) gateway.ToolRepoPort       { return r }
func ProvideGatewayPolicyRepo(r *policy.Repository) gateway.PolicyRepoPort { return r }
func ProvideGatewayAuditRepo(r *audit.Repository) gateway.AuditRepoPort    { return r }
func ProvideGatewayIdempotencyRepo(r *gateway.IdempotencyRepository) gateway.IdempotencyPort {
	return r
}
func ProvideGatewaySecretRepo(r *secrets.Repository) gateway.SecretRepoPort { return r }
func ProvideGatewayTenantCaps(r *admin.Repository) gateway.TenantLimitsPort { return r }
func ProvideGatewayEgressPort(s *egress.Usecases) gateway.EgressPort         { return s }
func ProvideGatewayRateLimiter(l ratelimit.Adapter) gateway.RateLimiterPort {
	return l
}
func ProvideGatewayMetrics(m *exectelemetry.RunMetrics) gateway.RunMetricsPort { return m }
func ProvideGatewayHTTPExecutor(e *exechttp.Executor) gateway.HTTPExecutorPort { return e }
func ProvideGatewayHandler(uc *gateway.Usecases) *gateway.Handler            { return gateway.NewHandler(uc) }

var GatewaySet = wire.NewSet(
	ProvideGatewayToolRepo,
	ProvideGatewayPolicyRepo,
	ProvideGatewayAuditRepo,
	ProvideGatewayIdempotencyRepo,
	ProvideGatewaySecretRepo,
	ProvideGatewayTenantCaps,
	ProvideGatewayEgressPort,
	ProvideGatewayRateLimiter,
	ProvideGatewayMetrics,
	ProvideGatewayHTTPExecutor,
	exectelemetry.NewRunMetrics,
	gateway.NewIdempotencyRepository,
	gateway.NewUsecases,
	ProvideGatewayHandler,
)
