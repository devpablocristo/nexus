package main

import (
	"log"
	"net/http"
	"os"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	"github.com/devpablocristo/nexus/v2/pkgs/go-pkg/httpserver"
	"nexus/v2/control-plane/wire"
)

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8081"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	handler, cleanup, err := wire.NewServer(wire.Config{
		AuditDatabaseURL:        os.Getenv("NEXUS_AUDIT_DATABASE_URL"),
		ControlPlaneDatabaseURL: os.Getenv("NEXUS_CONTROL_PLANE_DATABASE_URL"),
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

	log.Printf("control-plane v2 listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
