package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"nexus-gateway/cmd/config"
	"nexus-gateway/wire"
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
		_ = runServer(app.Server)
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	shutdownServer(app.Server)
}
