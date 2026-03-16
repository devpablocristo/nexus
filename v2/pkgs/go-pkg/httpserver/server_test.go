package httpserver

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Parallel()

	server := New(":8080", http.NewServeMux())

	if server.Addr != ":8080" {
		t.Fatalf("unexpected addr: %q", server.Addr)
	}
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("unexpected read header timeout: %s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 15*time.Second {
		t.Fatalf("unexpected read timeout: %s", server.ReadTimeout)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Fatalf("unexpected write timeout: %s", server.WriteTimeout)
	}
	if server.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected idle timeout: %s", server.IdleTimeout)
	}
}

func TestServeRejectsNilServer(t *testing.T) {
	t.Parallel()

	if err := Serve(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil server")
	}
}
