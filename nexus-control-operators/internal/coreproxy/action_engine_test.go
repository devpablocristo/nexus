package coreproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	opsaction "nexus-control-operators/internal/ops/actionengine"
)

func TestCoreActionEngine_ApplyForwardsLeaseHeadersFromDryRun(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	var seenAuth string
	var seenLeaseID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenLeaseID = r.Header.Get("X-Nexus-Lease-Id")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())
	client.retryBackoffBase = time.Millisecond
	engine := NewCoreActionEngine(client, t.TempDir())

	dryRun, err := engine.DryRun(context.Background(), orgID, nil, opsaction.EngineRequest{
		ActionType: "set_rate_limit",
		Scope: map[string]any{
			"level":  "org",
			"org_id": orgID.String(),
		},
		Params: map[string]any{
			"rpm": 100,
		},
		LeaseHeaders: map[string]string{
			"Authorization":    "Bearer lease-token",
			"X-Nexus-Lease-Id": "lease-1",
		},
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	_, err = engine.Apply(context.Background(), orgID, nil, opsaction.EngineRequest{
		ProposalID: &dryRun.Proposal.ID,
	})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if seenAuth != "Bearer lease-token" {
		t.Fatalf("expected lease auth header, got %q", seenAuth)
	}
	if seenLeaseID != "lease-1" {
		t.Fatalf("expected lease id header, got %q", seenLeaseID)
	}
}
