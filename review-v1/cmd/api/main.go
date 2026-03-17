package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devpablocristo/nexus/v2/pkgs/go-pkg/httpserver"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
	"github.com/devpablocristo/nexus/review-v1/wire"
)

func main() {
	logger := sharedobservability.NewJSONLogger("nexus-review")
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	cfg := wire.Config{
		APIKeys:      os.Getenv("NEXUS_REVIEW_API_KEYS"),
		ApprovalTTL:  time.Hour,
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
	}
	if cfg.APIKeys == "" {
		cfg.APIKeys = "nxr_dev=dev-key-change-me"
	}

	handler, err := wire.NewServer(cfg)
	if err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}

	metrics := sharedobservability.NewMetrics()
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
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
