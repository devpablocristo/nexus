package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/devpablocristo/nexus/v2/pkgs/go-pkg/httpserver"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	"nexus/v2/control-plane/wire"
)

func main() {
	logger := sharedobservability.NewJSONLogger("control-plane")
	metrics := sharedobservability.NewMetrics()
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8081"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	controlPlanePostgresConfig, err := sharedpostgres.ConfigFromEnv("NEXUS_CONTROL_PLANE_DB", "nexus-control-plane")
	if err != nil {
		logger.Error("control-plane postgres configuration failed", "error", err)
		os.Exit(1)
	}
	auditPostgresConfig, err := sharedpostgres.ConfigFromEnv("NEXUS_AUDIT_DB", "nexus-control-plane-audit")
	if err != nil {
		logger.Error("audit postgres configuration failed", "error", err)
		os.Exit(1)
	}

	handler, cleanup, err := wire.NewServer(wire.Config{
		AuditDatabaseURL:           os.Getenv("NEXUS_AUDIT_DATABASE_URL"),
		ControlPlaneDatabaseURL:    os.Getenv("NEXUS_CONTROL_PLANE_DATABASE_URL"),
		AuditPostgresConfig:        auditPostgresConfig,
		ControlPlanePostgresConfig: controlPlanePostgresConfig,
		NexusAPIKeys:               os.Getenv("NEXUS_API_KEYS"),
		SaaS: wire.SaaSConfig{
			DatabaseURL:           os.Getenv("NEXUS_SAAS_DATABASE_URL"),
			StripeSecretKey:       os.Getenv("STRIPE_SECRET_KEY"),
			StripeWebhookSecret:   os.Getenv("STRIPE_WEBHOOK_SECRET"),
			StripePriceStarter:    os.Getenv("STRIPE_PRICE_STARTER"),
			StripePriceGrowth:     os.Getenv("STRIPE_PRICE_GROWTH"),
			StripePriceEnterprise: os.Getenv("STRIPE_PRICE_ENTERPRISE"),
			TowerBaseURL:          os.Getenv("TOWER_BASE_URL"),
			ClerkWebhookSecret:    os.Getenv("CLERK_WEBHOOK_SECRET"),
			JWTIssuer:             os.Getenv("NEXUS_SAAS_JWT_ISSUER"),
			JWTAudience:           os.Getenv("NEXUS_SAAS_JWT_AUDIENCE"),
			JWTOrgClaim:           os.Getenv("NEXUS_SAAS_JWT_ORG_CLAIM"),
			JWTRoleClaim:          os.Getenv("NEXUS_SAAS_JWT_ROLE_CLAIM"),
			JWTScopesClaim:        os.Getenv("NEXUS_SAAS_JWT_SCOPES_CLAIM"),
			JWTActorClaim:         os.Getenv("NEXUS_SAAS_JWT_ACTOR_CLAIM"),
		},
	})
	if err != nil {
		logger.Error("control-plane startup failed", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	appHandler := sharedobservability.WithMetricsEndpoint(handler, metrics.Handler())
	securedHandler := httpserver.SecurityMiddleware(
		httpserver.SecurityConfigFromEnv(),
		sharedobservability.MiddlewareWithMetrics(logger, metrics, appHandler),
	)
	server := httpserver.New(addr, securedHandler)

	logger.Info("http server listening", "addr", addr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := httpserver.Serve(ctx, server, logger); err != nil && err != http.ErrServerClosed {
		logger.Error("http server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
