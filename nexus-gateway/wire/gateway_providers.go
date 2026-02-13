package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/audit"
	"nexus-gateway/internal/egress"
	"nexus-gateway/internal/gateway"
	exechttp "nexus-gateway/internal/gateway/executor/http"
	"nexus-gateway/internal/gateway/executor/ratelimit"
	exectelemetry "nexus-gateway/internal/gateway/executor/telemetry"
	"nexus-gateway/internal/policy"
	"nexus-gateway/internal/secrets"
	"nexus-gateway/internal/tool"
)

func ProvideGatewayToolRepo(r *tool.Repository) gateway.ToolRepoPort       { return r }
func ProvideGatewayPolicyRepo(r *policy.Repository) gateway.PolicyRepoPort { return r }
func ProvideGatewayAuditRepo(r *audit.Repository) gateway.AuditRepoPort    { return r }
func ProvideGatewayIdempotencyRepo(r *gateway.IdempotencyRepository) gateway.IdempotencyPort {
	return r
}
func ProvideGatewaySecretRepo(r *secrets.Repository) gateway.SecretRepoPort { return r }
func ProvideGatewayEgressPort(s egress.Service) gateway.EgressPort          { return s }
func ProvideGatewayRateLimiter(l ratelimit.Adapter) gateway.RateLimiterPort {
	return l
}
func ProvideGatewayMetrics(m *exectelemetry.RunMetrics) gateway.RunMetricsPort { return m }
func ProvideGatewayHTTPExecutor(e *exechttp.Executor) gateway.HTTPExecutorPort { return e }

var GatewaySet = wire.NewSet(
	ProvideGatewayToolRepo,
	ProvideGatewayPolicyRepo,
	ProvideGatewayAuditRepo,
	ProvideGatewayIdempotencyRepo,
	ProvideGatewaySecretRepo,
	ProvideGatewayEgressPort,
	ProvideGatewayRateLimiter,
	ProvideGatewayMetrics,
	ProvideGatewayHTTPExecutor,
	exectelemetry.NewRunMetrics,
	gateway.NewIdempotencyRepository,
	gateway.NewService,
	gateway.NewHandler,
)
