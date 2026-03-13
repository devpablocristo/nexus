package nexus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRunToolSendsHeadersAndParsesResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/run" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-NEXUS-CORE-KEY"); got != "test-key" {
			t.Fatalf("expected API key header, got %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "idem-1" {
			t.Fatalf("expected idempotency header, got %q", got)
		}
		if got := r.Header.Get("X-Timeout-Ms"); got != "750" {
			t.Fatalf("expected timeout header, got %q", got)
		}

		var body RunRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.ToolName != "echo" {
			t.Fatalf("expected tool echo, got %s", body.ToolName)
		}

		_ = json.NewEncoder(w).Encode(RunResponse{
			RequestID: "req-1",
			Decision:  "allow",
			ToolName:  "echo",
			Status:    "success",
			Result: map[string]any{
				"ok": true,
			},
			LatencyMS: 12,
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	resp, err := client.RunTool(context.Background(), RunRequest{
		ToolName: "echo",
		Input:    map[string]any{"message": "hello"},
		Headers: map[string]string{
			"Idempotency-Key": "idem-1",
			"X-Timeout-Ms":    "750",
		},
	})
	if err != nil {
		t.Fatalf("RunTool error: %v", err)
	}
	if resp.Status != "success" || resp.RequestID != "req-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestClientListToolsReturnsItems(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []Tool{
				{
					ID:         "tool-1",
					Name:       "echo",
					Kind:       "http",
					Method:     "POST",
					URL:        "http://example.test/echo",
					ActionType: "read",
					Enabled:    true,
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	items, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "echo" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestClientReturnsAPIErrorEnvelope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "POLICY_DENIED",
				"message": "blocked by policy",
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	_, err := client.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "POLICY_DENIED: blocked by policy") {
		t.Fatalf("unexpected error: %v", err)
	}
}
