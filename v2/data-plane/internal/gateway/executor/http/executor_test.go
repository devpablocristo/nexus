package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExecutePostJSONAndHeaders(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPayload map[string]any
	var gotRequestID string
	var gotAuth string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotRequestID = r.Header.Get("X-Nexus-Request-Id")
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"received": gotPayload,
		})
	}))
	defer upstream.Close()

	executor := NewExecutor(2 * time.Second)
	result, err := executor.Execute(context.Background(), http.MethodPost, upstream.URL, map[string]any{
		"hello": "world",
	}, map[string]string{
		"X-Nexus-Request-Id": "req-123",
		"Authorization":      "Bearer token",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotRequestID != "req-123" {
		t.Fatalf("unexpected request id header: %q", gotRequestID)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotPayload["hello"] != "world" {
		t.Fatalf("unexpected payload: %#v", gotPayload)
	}

	body, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	received, ok := body["received"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected received type: %T", body["received"])
	}
	if received["hello"] != "world" {
		t.Fatalf("unexpected response payload: %#v", received)
	}
}

func TestExecuteGetQueryParams(t *testing.T) {
	t.Parallel()

	var gotQuery string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
		})
	}))
	defer upstream.Close()

	executor := NewExecutor(2 * time.Second)
	result, err := executor.Execute(context.Background(), http.MethodGet, upstream.URL, map[string]any{
		"hello": "world",
		"n":     3,
	}, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if gotQuery != "hello=world&n=3" && gotQuery != "n=3&hello=world" {
		t.Fatalf("unexpected query: %q", gotQuery)
	}

	body, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if body["ok"] != true {
		t.Fatalf("unexpected response body: %#v", body)
	}
}

func TestExecuteReturnsRawForNonJSON(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("pong"))
	}))
	defer upstream.Close()

	executor := NewExecutor(2 * time.Second)
	result, err := executor.Execute(context.Background(), http.MethodGet, upstream.URL, map[string]any{}, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	body, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if body["raw"] != "pong" {
		t.Fatalf("unexpected raw body: %#v", body)
	}
}

func TestExecuteFailsOnNon2xx(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer upstream.Close()

	executor := NewExecutor(2 * time.Second)
	_, err := executor.Execute(context.Background(), http.MethodGet, upstream.URL, map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
