package saasclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestListRestoreEvidence_FallbackOnFailure(t *testing.T) {
	t.Setenv("NEXUS_SAAS_URL", "http://127.0.0.1:1")
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "k1")
	c := NewRestoreEvidenceClient(zerolog.Nop())
	items, err := c.ListRestoreEvidence(context.Background(), uuid.New(), "prod", 5)
	if err != nil {
		t.Fatalf("expected nil error fallback, got %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty fallback, got %d items", len(items))
	}
}

func TestListRestoreEvidence_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-NEXUS-SAAS-KEY") != "k1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"` + uuid.NewString() + `","environment":"prod","system":"database","status":"passed","snapshot_id":"snap-123","restore_target":"restore-temp","completed_at":"2026-02-19T12:00:00Z","source":"dr.test_restore.sh","artifact_sha256":"sha-1","summary":{"core_ok":true},"created_at":"2026-02-19T12:01:00Z"}]}`))
	}))
	defer srv.Close()

	t.Setenv("NEXUS_SAAS_URL", srv.URL)
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "k1")
	c := NewRestoreEvidenceClient(zerolog.Nop())
	items, err := c.ListRestoreEvidence(context.Background(), uuid.New(), "prod", 5)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(items) != 1 || items[0].SnapshotID != "snap-123" {
		t.Fatalf("unexpected items: %#v", items)
	}
}
