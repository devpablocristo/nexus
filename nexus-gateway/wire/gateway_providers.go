package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/audit"
	"nexus-gateway/internal/gateway"
	exechttp "nexus-gateway/internal/gateway/executor/http"
	"nexus-gateway/internal/gateway/executor/ratelimit"
	"nexus-gateway/internal/policy"
	"nexus-gateway/internal/tool"
)

func ProvideGatewayToolRepo(r *tool.Repository) gateway.ToolRepoPort       { return r }
func ProvideGatewayPolicyRepo(r *policy.Repository) gateway.PolicyRepoPort { return r }
func ProvideGatewayAuditRepo(r *audit.Repository) gateway.AuditRepoPort    { return r }
func ProvideGatewayRateLimiter(l *ratelimit.Limiter) gateway.RateLimiterPort {
	return l
}
func ProvideGatewayHTTPExecutor(e *exechttp.Executor) gateway.HTTPExecutorPort { return e }

var GatewaySet = wire.NewSet(
	ProvideGatewayToolRepo,
	ProvideGatewayPolicyRepo,
	ProvideGatewayAuditRepo,
	ProvideGatewayRateLimiter,
	ProvideGatewayHTTPExecutor,
	gateway.NewService,
	gateway.NewHandler,
)
