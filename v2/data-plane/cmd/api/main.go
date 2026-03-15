package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"nexus/v2/data-plane/wire"
)

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	cfg := wire.Config{
		EchoURL:           os.Getenv("NEXUS_TOOL_ECHO_URL"),
		ControlPlaneURL:   os.Getenv("NEXUS_CONTROL_PLANE_URL"),
		ControlWorkersURL: os.Getenv("NEXUS_CONTROL_WORKERS_URL"),
		HTTPTimeout:       5 * time.Second,
		RateLimitBackend:  os.Getenv("NEXUS_RATE_LIMIT_BACKEND"),
		RedisURL:          os.Getenv("NEXUS_REDIS_URL"),
	}
	handler, cleanup, err := wire.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("data-plane v2 listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
