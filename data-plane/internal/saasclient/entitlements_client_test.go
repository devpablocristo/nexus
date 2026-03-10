package saasclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"nexus/pkg/types"
)

func TestGetRunRPM_FallbackOnFailure(t *testing.T) {
	t.Setenv("NEXUS_SAAS_URL", "http://127.0.0.1:1")
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "k1")
	c := NewEntitlementsClient(zerolog.Nop())
	rpm, err := c.GetRunRPM(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("expected nil error fallback, got %v", err)
	}
	if rpm != 0 {
		t.Fatalf("expected fallback 0, got %d", rpm)
	}
}

func TestGetRunRPM_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-NEXUS-SAAS-KEY") != "k1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"org_id":"` + uuid.NewString() + `","plan_code":"growth","hard_limits":{"run_rpm":1200}}`))
	}))
	defer srv.Close()

	t.Setenv("NEXUS_SAAS_URL", srv.URL)
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "k1")
	c := NewEntitlementsClient(zerolog.Nop())
	rpm, err := c.GetRunRPM(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rpm != 1200 {
		t.Fatalf("expected 1200, got %d", rpm)
	}
}

func TestGetRunRPM_ReturnsForbiddenWhenTenantSuspended(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-NEXUS-SAAS-KEY") != "k1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"org_id":"` + uuid.NewString() + `","plan_code":"growth","status":"suspended","hard_limits":{"run_rpm":1200}}`))
	}))
	defer srv.Close()

	t.Setenv("NEXUS_SAAS_URL", srv.URL)
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "k1")
	c := NewEntitlementsClient(zerolog.Nop())
	_, err := c.GetRunRPM(context.Background(), uuid.New())
	if err == nil {
		t.Fatalf("expected forbidden error")
	}
	httpErr, ok := err.(types.HTTPError)
	if !ok {
		t.Fatalf("expected types.HTTPError, got %T", err)
	}
	if httpErr.Status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", httpErr.Status)
	}
}
