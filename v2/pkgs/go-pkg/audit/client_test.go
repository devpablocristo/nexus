package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientCreate(t *testing.T) {
	t.Parallel()

	var got WriteRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/internal/audit" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got, want := r.Header.Get("X-API-Key"), "audit-secret"; got != want {
			t.Fatalf("unexpected api key header: got=%q want=%q", got, want)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second).WithAPIKey("audit-secret")
	err := client.Create(context.Background(), WriteRequest{
		EventType:     "action_created",
		SourceService: "data-plane",
		ActionID:      "action-1",
		ResourceID:    "resource-1",
		ResourceType:  "wallet",
		Actor:         &Actor{Type: "system", ID: "treasury-bot"},
		Summary:       "withdrawal created",
		Data:          map[string]any{"status": "pending_approval"},
		OccurredAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if got.EventType != "action_created" || got.SourceService != "data-plane" || got.ActionID != "action-1" {
		t.Fatalf("unexpected audit payload: %#v", got)
	}
}

func TestClientCreateStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	err := client.Create(context.Background(), WriteRequest{
		EventType:     "action_created",
		SourceService: "data-plane",
		Summary:       "withdrawal created",
	})
	if err == nil {
		t.Fatal("expected status error")
	}
}
