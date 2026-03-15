package action

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

func TestControlWorkersClientCreate(t *testing.T) {
	t.Parallel()

	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/incidents" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"incident-1"}`))
	}))
	defer server.Close()

	client := NewControlWorkersClient(server.URL, 0)
	err := client.Create(context.Background(), IncidentRequest{
		SourceID:     "action-1",
		ActionType:   actiondomain.ActionTypeWithdrawal,
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: actiondomain.ResourceTypeWallet,
		Trigger:      IncidentTriggerBlockedAction,
		RiskLevel:    actiondomain.RiskLevelHigh,
		Summary:      "withdrawal blocked by Nexus",
		Reason:       "blocked by policy",
		Details:      map[string]any{"decision": "deny"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got["source_kind"] != "action" || got["source_id"] != "action-1" || got["trigger"] != "blocked_action" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}
