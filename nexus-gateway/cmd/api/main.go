package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"nexus-gateway/cmd/config"
	"nexus-gateway/pkg/telemetry"
	"nexus-gateway/wire"
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

	go func() {
		_ = runServer(app.Server)
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	shutdownServer(app.Server)
}
