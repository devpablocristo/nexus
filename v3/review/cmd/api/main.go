package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/devpablocristo/core/backend/go/httpserver"
	sharedobservability "github.com/devpablocristo/core/backend/go/observability"
	"github.com/devpablocristo/nexus/v3/review/migrations"
	"github.com/devpablocristo/nexus/v3/review/wire"
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

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	approvalTTL := time.Hour
	if ttlStr := os.Getenv("APPROVAL_DEFAULT_TTL"); ttlStr != "" {
		if secs, err := strconv.Atoi(ttlStr); err == nil && secs > 0 {
			approvalTTL = time.Duration(secs) * time.Second
		}
	}

	cfg := wire.Config{
		DatabaseURL:    databaseURL,
		APIKeys:        os.Getenv("NEXUS_API_KEYS"),
		ApprovalTTL:    approvalTTL,
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		SigningKey:     os.Getenv("NEXUS_SIGNING_KEY"),
		MigrationFiles: migrations.Files,
	}
	if cfg.APIKeys == "" {
		logger.Error("NEXUS_API_KEYS is required")
		os.Exit(1)
	}

	handler, cleanup, err := wire.NewServer(cfg)
	if err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	// Limitar tamaño de request body a 1MB
	const maxBodySize = 1 << 20
	limitedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		handler.ServeHTTP(w, r)
	})

	metrics := sharedobservability.NewMetrics(sharedobservability.DefaultMetricsConfig("nexus_review"))
	appHandler := sharedobservability.WithMetricsEndpoint(limitedHandler, metrics.Handler())
	securedHandler := httpserver.SecurityMiddleware(
		httpserver.SecurityConfigFromEnv("NEXUS"),
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
