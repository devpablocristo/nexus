package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"nexus/v2/control-workers/wire"
)

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8082"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           wire.NewServer(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("control-workers v2 listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
