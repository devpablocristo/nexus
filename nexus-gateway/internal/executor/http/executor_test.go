package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nexus-gateway/pkg/types"
)

func TestExecutor_GETRejectsNestedInput(t *testing.T) {
	ex := NewExecutor(Options{Timeout: 2 * time.Second, MaxResponseBytes: 1024, Retries: 0})
	_, _, he := ex.Execute(context.Background(), "GET", "http://example.invalid", map[string]any{"a": map[string]any{"b": 1}})
	if he == nil || he.Code != types.ErrCodeInvalidGETInput {
		t.Fatalf("expected %s, got %#v", types.ErrCodeInvalidGETInput, he)
	}
}

func TestExecutor_ResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"x":"` + strings.Repeat("a", 50) + `"}`))
	}))
	t.Cleanup(srv.Close)

	ex := NewExecutor(Options{Timeout: 2 * time.Second, MaxResponseBytes: 10, Retries: 0})
	_, _, he := ex.Execute(context.Background(), "POST", srv.URL, map[string]any{"a": 1})
	if he == nil || he.Code != types.ErrCodeResponseTooLarge {
		t.Fatalf("expected %s, got %#v", types.ErrCodeResponseTooLarge, he)
	}
}

func TestExecutor_NonJSONReturnsRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	t.Cleanup(srv.Close)

	ex := NewExecutor(Options{Timeout: 2 * time.Second, MaxResponseBytes: 1024, Retries: 0})
	res, status, he := ex.Execute(context.Background(), "POST", srv.URL, map[string]any{"a": 1})
	if he != nil {
		t.Fatalf("unexpected error: %#v", he)
	}
	if status != 200 {
		t.Fatalf("expected 200 got %d", status)
	}
	m, ok := res.(map[string]any)
	if !ok || m["raw"] != "hello" {
		b, _ := json.Marshal(res)
		t.Fatalf("expected raw hello, got %s", string(b))
	}
}
