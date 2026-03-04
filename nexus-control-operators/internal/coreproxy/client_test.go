package coreproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestClient_DoJSON_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-NEXUS-AI-KEY") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": 42})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())

	var out map[string]any
	err := client.DoJSON(context.Background(), "POST", "/test", map[string]string{"foo": "bar"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok=true, got %v", out["ok"])
	}
}

func TestClient_DoJSON_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())

	var out map[string]any
	err := client.DoJSON(context.Background(), "GET", "/fail", nil, &out)
	if err == nil {
		t.Fatalf("expected error for 500 response")
	}
}

func TestClient_Ping_OK(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key", 2*time.Second, zerolog.Nop())
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("ping should succeed: %v", err)
	}
}

func TestClient_Ping_Down(t *testing.T) {
	t.Parallel()

	client := NewClient("http://127.0.0.1:1", "key", 1*time.Second, zerolog.Nop())
	if err := client.Ping(context.Background()); err == nil {
		t.Fatalf("ping should fail for unreachable server")
	}
}
