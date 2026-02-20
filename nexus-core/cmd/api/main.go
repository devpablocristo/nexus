package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nexus-core/cmd/config"
	"nexus-core/pkg/telemetry"
	"nexus-core/wire"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	telemetryShutdown, err := telemetry.Init(context.Background(), telemetry.Config{
		Enabled:     cfg.Service.OTelEnabled,
		ServiceName: cfg.Service.OTelServiceName,
		Endpoint:    cfg.Service.OTLPEndpoint,
		Insecure:    cfg.Service.OTLPInsecure,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = telemetryShutdown(context.Background())
	}()

	app, cleanup, err := wire.InitializeAPI(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if app.ActionTTLEngine != nil {
		go runActionTTLSweep(ctx, app.ActionTTLEngine)
	}

	go func() {
		_ = runServer(app.Server)
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	cancel()

	shutdownServer(app.Server)
}

func runActionTTLSweep(ctx context.Context, engine wire.ActionTTLEngine) {
	interval := envInt("NEXUS_ACTION_TTL_SWEEP_SECONDS", 15)
	batch := envInt("NEXUS_ACTION_TTL_SWEEP_BATCH", 200)
	if interval <= 0 {
		interval = 15
	}
	if batch <= 0 {
		batch = 200
	}
	t := time.NewTicker(time.Duration(interval) * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if _, err := engine.ExpireDue(ctx, now.UTC(), batch); err != nil {
				fmt.Fprintf(os.Stderr, "action ttl sweep error: %v\n", err)
			}
		}
	}
}

func envInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
