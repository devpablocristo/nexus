package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"nexus-saas/cmd/config"
	"nexus-saas/wire"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	app, cleanup, err := wire.InitializeAPI(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer cleanup()

	go func() {
		_ = app.Server.ListenAndServe()
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
	defer cancel()
	_ = app.Server.Shutdown(ctx)
}
