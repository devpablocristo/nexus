package main

import (
	"log"
	"net/http"
	"os"
	"time"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	"github.com/devpablocristo/nexus/v2/pkgs/go-pkg/httpserver"
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

	handler, cleanup, err := wire.NewServer(wire.Config{
		ControlPlaneURL:           os.Getenv("NEXUS_CONTROL_PLANE_URL"),
		ControlPlaneAPIKey:        os.Getenv("NEXUS_CONTROL_PLANE_API_KEY"),
		ControlWorkersDatabaseURL: os.Getenv("NEXUS_CONTROL_WORKERS_DATABASE_URL"),
		HTTPTimeout:               5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	authn, err := sharedapikey.NewAuthenticator(os.Getenv("NEXUS_API_KEYS"))
	if err != nil {
		log.Fatal(err)
	}

	server := httpserver.New(addr, authn.Middleware(handler))

	log.Printf("control-workers v2 listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
