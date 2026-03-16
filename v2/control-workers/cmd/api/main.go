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
	"nexus/v2/control-workers/wire"
)

func main() {
	logger := sharedobservability.NewJSONLogger("control-workers")
	metrics := sharedobservability.NewMetrics()
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8082"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	controlWorkersPostgresConfig, err := sharedpostgres.ConfigFromEnv("NEXUS_CONTROL_WORKERS_DB", "nexus-control-workers")
	if err != nil {
		logger.Error("control-workers postgres configuration failed", "error", err)
		os.Exit(1)
	}

	handler, cleanup, err := wire.NewServer(wire.Config{
		ControlPlaneURL:              os.Getenv("NEXUS_CONTROL_PLANE_URL"),
		ControlPlaneAPIKey:           os.Getenv("NEXUS_CONTROL_PLANE_API_KEY"),
		ControlWorkersDatabaseURL:    os.Getenv("NEXUS_CONTROL_WORKERS_DATABASE_URL"),
		HTTPTimeout:                  5 * time.Second,
		Metrics:                      metrics,
		ControlWorkersPostgresConfig: controlWorkersPostgresConfig,
	})
	if err != nil {
		logger.Error("control-workers startup failed", "error", err)
		os.Exit(1)
	}
	defer cleanup()
	authn, err := sharedapikey.NewAuthenticator(os.Getenv("NEXUS_API_KEYS"))
	if err != nil {
		logger.Error("control-workers auth configuration failed", "error", err)
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
