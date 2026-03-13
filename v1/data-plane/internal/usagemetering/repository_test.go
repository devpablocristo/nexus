package usagemetering

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestIncrement_NoConfig_IsNoop(t *testing.T) {
	t.Setenv("NEXUS_SAAS_URL", "")
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "")
	repo := NewRepository(nil)
	if err := repo.Increment(context.Background(), uuid.New(), CounterAPICalls); err != nil {
		t.Fatalf("expected noop, got err: %v", err)
	}
}

func TestIncrement_ForwardsToSaaS(t *testing.T) {
	var seenKey string
	var seenCounter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/usage/events" {
			http.NotFound(w, r)
			return
		}
		seenKey = r.Header.Get("X-NEXUS-SAAS-KEY")
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if v, ok := payload["counter"].(string); ok {
			seenCounter = v
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	t.Setenv("NEXUS_SAAS_URL", srv.URL)
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "int-key")
	t.Setenv("NEXUS_SAAS_TIMEOUT_MS", "1000")
	repo := NewRepository(nil)
	if err := repo.Increment(context.Background(), uuid.New(), CounterAPICalls); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if seenKey != "int-key" {
		t.Fatalf("expected internal key, got %q", seenKey)
	}
	if seenCounter != CounterAPICalls {
		t.Fatalf("expected counter %q, got %q", CounterAPICalls, seenCounter)
	}
}
