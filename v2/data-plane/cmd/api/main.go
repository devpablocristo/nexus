package main

import (
	"log"
	"net/http"
	"os"
	"time"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	"github.com/devpablocristo/nexus/v2/pkgs/go-pkg/httpserver"
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
		ControlPlaneURL:      os.Getenv("NEXUS_CONTROL_PLANE_URL"),
		ControlPlaneAPIKey:   os.Getenv("NEXUS_CONTROL_PLANE_API_KEY"),
		ControlWorkersURL:    os.Getenv("NEXUS_CONTROL_WORKERS_URL"),
		ControlWorkersAPIKey: os.Getenv("NEXUS_CONTROL_WORKERS_API_KEY"),
		DataPlaneDatabaseURL: os.Getenv("NEXUS_DATA_PLANE_DATABASE_URL"),
		HTTPTimeout:          5 * time.Second,
	}
	handler, cleanup, err := wire.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	authn, err := sharedapikey.NewAuthenticator(os.Getenv("NEXUS_API_KEYS"))
	if err != nil {
		log.Fatal(err)
	}

	server := httpserver.New(addr, authn.Middleware(handler))

	log.Printf("data-plane v2 listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
