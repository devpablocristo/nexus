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

func TestCoreActionEngine_ApplyForwardsStoredLeaseHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/operators/actions/apply" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("X-NEXUS-AI-KEY"); got != "test-key" {
			t.Fatalf("expected api key header, got %q", got)
		}
		if got := r.Header.Get("X-Nexus-Execution-Token"); got != "lease-token" {
			t.Fatalf("expected execution token header, got %q", got)
		}
		if got := r.Header.Get("X-Nexus-Lease-Id"); got != "lease-1" {
			t.Fatalf("expected lease id header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())
	engine := NewCoreActionEngine(client, "")
	orgID := uuid.New()
	actor := "agents.mitigation"

	dryRun, err := engine.DryRun(context.Background(), orgID, &actor, opsaction.EngineRequest{
		ActionType: "set_rate_limit",
		Scope:      map[string]any{"level": "org", "org_id": orgID.String()},
		Params:     map[string]any{"rpm": 120},
		LeaseHeaders: map[string]string{
			"X-Nexus-Execution-Token": "lease-token",
			"X-Nexus-Lease-Id":        "lease-1",
		},
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if _, err := engine.Apply(context.Background(), orgID, &actor, opsaction.EngineRequest{
		ProposalID: &dryRun.Proposal.ID,
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
}

func TestCoreActionEngine_ApplyOverlayLeaseHeadersWin(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Nexus-Execution-Token"); got != "overlay-token" {
			t.Fatalf("expected overlay execution token header, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer overlay-token" {
			t.Fatalf("expected overlay authorization header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 2*time.Second, zerolog.Nop())
	engine := NewCoreActionEngine(client, "")
	orgID := uuid.New()
	actor := "agents.mitigation"

	dryRun, err := engine.DryRun(context.Background(), orgID, &actor, opsaction.EngineRequest{
		ActionType: "set_rate_limit",
		Scope:      map[string]any{"level": "org", "org_id": orgID.String()},
		Params:     map[string]any{"rpm": 120},
		LeaseHeaders: map[string]string{
			"X-Nexus-Execution-Token": "stored-token",
			"Authorization":           "Bearer stored-token",
		},
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if _, err := engine.Apply(context.Background(), orgID, &actor, opsaction.EngineRequest{
		ProposalID: &dryRun.Proposal.ID,
		LeaseHeaders: map[string]string{
			"X-Nexus-Execution-Token": "overlay-token",
			"Authorization":           "Bearer overlay-token",
		},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
}
