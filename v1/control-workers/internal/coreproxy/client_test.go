package coreproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
	client.retryBackoffBase = time.Millisecond

	var out map[string]any
	err := client.DoJSON(context.Background(), "POST", "/test", map[string]string{"foo": "bar"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok=true, got %v", out["ok"])
	}
}

func TestClient_DoJSON_ForwardsExecutionLeaseHeadersFromContext(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-NEXUS-AI-KEY") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer lease-token" {
			t.Fatalf("expected Authorization header, got %q", got)
		}
		if got := r.Header.Get("X-Nexus-Execution-Token"); got != "lease-token" {
			t.Fatalf("expected execution token header, got %q", got)
		}
		if got := r.Header.Get("X-Nexus-Lease-Id"); got != "lease-1" {
			t.Fatalf("expected lease id header, got %q", got)
		}
		if got := r.Header.Get("X-Nexus-Credential-Mode"); got != "aws_sts" {
			t.Fatalf("expected credential mode header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())
	client.retryBackoffBase = time.Millisecond

	ctx := WithExecutionLeaseHeaders(context.Background(), map[string]string{
		"Authorization":           "Bearer lease-token",
		"X-Nexus-Execution-Token": "lease-token",
		"X-Nexus-Lease-Id":        "lease-1",
		"X-Nexus-Credential-Mode": "aws_sts",
		"X-Unrelated":             "ignored",
	})

	var out map[string]any
	err := client.DoJSON(ctx, "POST", "/test", map[string]string{"foo": "bar"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DoJSON_IgnoresEmptyExecutionLeaseHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Nexus-Execution-Token"); got != "" {
			t.Fatalf("expected empty execution token header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())
	client.retryBackoffBase = time.Millisecond

	ctx := WithExecutionLeaseHeaders(context.Background(), map[string]string{
		"X-Nexus-Execution-Token": "   ",
	})

	var out map[string]any
	err := client.DoJSON(ctx, "POST", "/test", map[string]string{"foo": "bar"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	client.retryBackoffBase = time.Millisecond

	var out map[string]any
	err := client.DoJSON(context.Background(), "GET", "/fail", nil, &out)
	if err == nil {
		t.Fatalf("expected error for 500 response")
	}
}

func TestClient_DoJSON_RetryAndRecover(t *testing.T) {
	t.Parallel()

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"temporary"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())
	client.retryBackoffBase = time.Millisecond
	client.retryAttempts = 3

	var out map[string]any
	err := client.DoJSON(context.Background(), "GET", "/retry", nil, &out)
	if err != nil {
		t.Fatalf("expected retry to recover: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
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
