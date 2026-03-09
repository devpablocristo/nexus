package saasclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestListProtectedResources_FallbackOnFailure(t *testing.T) {
	t.Setenv("NEXUS_SAAS_URL", "http://127.0.0.1:1")
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "k1")
	c := NewProtectedResourcesClient(zerolog.Nop())
	items, err := c.ListProtectedResources(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("expected nil error fallback, got %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty fallback, got %d items", len(items))
	}
}

func TestListProtectedResources_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-NEXUS-SAAS-KEY") != "k1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"` + uuid.NewString() + `","name":"prod-db","resource_type":"host","match_value":"db-prod.internal","match_mode":"contains","environment":"prod","reason":"primary database"}]}`))
	}))
	defer srv.Close()

	t.Setenv("NEXUS_SAAS_URL", srv.URL)
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "k1")
	c := NewProtectedResourcesClient(zerolog.Nop())
	items, err := c.ListProtectedResources(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(items) != 1 || items[0].Name != "prod-db" {
		t.Fatalf("unexpected items: %#v", items)
	}
}
