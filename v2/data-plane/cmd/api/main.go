package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	"github.com/devpablocristo/nexus/v2/pkgs/go-pkg/httpserver"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	"nexus/v2/data-plane/wire"
)

func main() {
	logger := sharedobservability.NewJSONLogger("data-plane")
	metrics := sharedobservability.NewMetrics()
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	dataPlanePostgresConfig, err := sharedpostgres.ConfigFromEnv("NEXUS_DATA_PLANE_DB", "nexus-data-plane")
	if err != nil {
		logger.Error("data-plane postgres configuration failed", "error", err)
		os.Exit(1)
	}

	cfg := wire.Config{
		ControlPlaneURL:         os.Getenv("NEXUS_CONTROL_PLANE_URL"),
		ControlPlaneAPIKey:      os.Getenv("NEXUS_CONTROL_PLANE_API_KEY"),
		ControlWorkersURL:       os.Getenv("NEXUS_CONTROL_WORKERS_URL"),
		ControlWorkersAPIKey:    os.Getenv("NEXUS_CONTROL_WORKERS_API_KEY"),
		DataPlaneDatabaseURL:    os.Getenv("NEXUS_DATA_PLANE_DATABASE_URL"),
		DataPlanePostgresConfig: dataPlanePostgresConfig,
		HTTPTimeout:             5 * time.Second,
		Metrics:                 metrics,
	}
	handler, cleanup, err := wire.NewServer(cfg)
	if err != nil {
		logger.Error("data-plane startup failed", "error", err)
		os.Exit(1)
	}
	defer cleanup()
	authn, err := sharedapikey.NewAuthenticator(os.Getenv("NEXUS_API_KEYS"))
	if err != nil {
		logger.Error("data-plane auth configuration failed", "error", err)
		os.Exit(1)
	}

	appHandler := sharedobservability.WithMetricsEndpoint(handler, metrics.Handler())
	securedHandler := httpserver.SecurityMiddleware(
		httpserver.SecurityConfigFromEnv(),
		sharedobservability.MiddlewareWithMetrics(logger, metrics, authn.Middleware(appHandler)),
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
