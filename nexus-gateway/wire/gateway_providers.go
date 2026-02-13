package wire

import (
	"github.com/google/wire"

	auditrepo "nexus-gateway/internal/audit/repository"
	exechttp "nexus-gateway/internal/executor/http"
	"nexus-gateway/internal/executor/ratelimit"
	gwhandler "nexus-gateway/internal/gateway/handler"
	gwuc "nexus-gateway/internal/gateway/usecases"
	policyrepo "nexus-gateway/internal/policy/repository"
	toolrepo "nexus-gateway/internal/tool/repository"
)

func ProvideGatewayToolRepo(r *toolrepo.Repository) gwuc.ToolRepoPort       { return r }
func ProvideGatewayPolicyRepo(r *policyrepo.Repository) gwuc.PolicyRepoPort { return r }
func ProvideGatewayAuditRepo(r *auditrepo.Repository) gwuc.AuditRepoPort    { return r }
func ProvideGatewayRateLimiter(l *ratelimit.Limiter) gwuc.RateLimiterPort   { return l }
func ProvideGatewayHTTPExecutor(e *exechttp.Executor) gwuc.HTTPExecutorPort { return e }

var GatewaySet = wire.NewSet(
	ProvideGatewayToolRepo,
	ProvideGatewayPolicyRepo,
	ProvideGatewayAuditRepo,
	ProvideGatewayRateLimiter,
	ProvideGatewayHTTPExecutor,
	gwuc.NewService,
	gwhandler.NewHandler,
)
