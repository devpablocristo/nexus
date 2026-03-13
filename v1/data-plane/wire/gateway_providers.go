package wire

import (
	"github.com/google/wire"
	"github.com/rs/zerolog"

	"data-plane/internal/admin"
	"data-plane/internal/audit"
	"data-plane/internal/dlp"
	"data-plane/internal/egress"
	"data-plane/internal/gateway"
	exechttp "data-plane/internal/gateway/executor/http"
	"data-plane/internal/gateway/executor/ratelimit"
	exectelemetry "data-plane/internal/gateway/executor/telemetry"
	"data-plane/internal/policy"
	"data-plane/internal/saasclient"
	"data-plane/internal/secrets"
	"data-plane/internal/tool"
	"nexus/pkg/validations/jsonschema"
)

func ProvideGatewayToolRepo(r *tool.Repository) gateway.ToolRepoPort       { return r }
func ProvideGatewayPolicyRepo(r *policy.Repository) gateway.PolicyRepoPort { return r }
func ProvideGatewayAuditRepo(r *audit.Repository) gateway.AuditRepoPort    { return r }
func ProvideGatewayIntentRepo(r *gateway.IntentRepository) gateway.IntentRepoPort {
	return r
}
func ProvideGatewayLeaseRepo(r *gateway.LeaseRepository) gateway.LeaseRepoPort { return r }
func ProvideGatewayLeaseCredentialBroker(b *gateway.LeaseMetadataBroker) gateway.LeaseCredentialBrokerPort {
	return b
}
func ProvideGatewayIdempotencyRepo(r *gateway.IdempotencyRepository) gateway.IdempotencyPort {
	return r
}
func ProvideGatewaySecretRepo(r *secrets.Repository) gateway.SecretRepoPort { return r }
func ProvideGatewayTenantCaps(_ *admin.Repository) gateway.TenantLimitsPort {
	return saasclient.NewEntitlementsClient(zerolog.Nop())
}
func ProvideGatewayProtectedResources(_ *admin.Repository) gateway.ProtectedResourcePort {
	return saasclient.NewProtectedResourcesClient(zerolog.Nop())
}
func ProvideGatewayRestoreEvidence(_ *admin.Repository) gateway.RestoreEvidencePort {
	return saasclient.NewRestoreEvidenceClient(zerolog.Nop())
}
func ProvideGatewayEgressPort(s *egress.Usecases) gateway.EgressPort { return s }
func ProvideGatewayRateLimiter(l ratelimit.Adapter) gateway.RateLimiterPort {
	return l
}
func ProvideGatewayMetrics(m *exectelemetry.RunMetrics) gateway.RunMetricsPort { return m }
func ProvideGatewayHTTPExecutor(e *exechttp.Executor) gateway.HTTPExecutorPort { return e }
func ProvideGatewayHandler(uc *gateway.Usecases) *gateway.Handler              { return gateway.NewHandler(uc) }
func ProvideGatewayUsecases(toolRepo gateway.ToolRepoPort, policyRepo gateway.PolicyRepoPort, auditRepo gateway.AuditRepoPort, secretRepo gateway.SecretRepoPort, egress gateway.EgressPort, limiter gateway.RateLimiterPort, executor gateway.HTTPExecutorPort, idempotency gateway.IdempotencyPort, tenantCaps gateway.TenantLimitsPort, actionOverrides gateway.ActionOverridesPort, protectedResources gateway.ProtectedResourcePort, restoreEvidence gateway.RestoreEvidencePort, intentRepo gateway.IntentRepoPort, leaseRepo gateway.LeaseRepoPort, leaseCreds gateway.LeaseCredentialBrokerPort, approval gateway.ApprovalPort, metrics gateway.RunMetricsPort, cache *jsonschema.CompilerCache, evaluator *policy.Evaluator, detector *dlp.Detector, cfg gateway.Config, log zerolog.Logger) *gateway.Usecases {
	return gateway.NewUsecases(toolRepo, policyRepo, auditRepo, secretRepo, egress, limiter, executor, idempotency, tenantCaps, actionOverrides, approval, metrics, cache, evaluator, detector, cfg, log).
		WithIntentRepo(intentRepo).
		WithLeaseRepo(leaseRepo).
		WithLeaseCredentialBroker(leaseCreds).
		WithProtectedResources(protectedResources).
		WithRestoreEvidence(restoreEvidence)
}

var GatewaySet = wire.NewSet(
	ProvideGatewayToolRepo,
	ProvideGatewayPolicyRepo,
	ProvideGatewayAuditRepo,
	gateway.NewIntentRepository,
	ProvideGatewayIntentRepo,
	gateway.NewLeaseRepository,
	ProvideGatewayLeaseRepo,
	gateway.NewLeaseMetadataBroker,
	ProvideGatewayLeaseCredentialBroker,
	ProvideGatewayIdempotencyRepo,
	ProvideGatewaySecretRepo,
	ProvideGatewayTenantCaps,
	ProvideGatewayProtectedResources,
	ProvideGatewayRestoreEvidence,
	ProvideGatewayEgressPort,
	ProvideGatewayRateLimiter,
	ProvideGatewayMetrics,
	ProvideGatewayHTTPExecutor,
	exectelemetry.NewRunMetrics,
	gateway.NewIdempotencyRepository,
	ProvideGatewayUsecases,
	ProvideGatewayHandler,
)
