package action

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
)

func TestControlPlaneClientGetByIDEscapesPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/v1/resources/wallet%2Fhot%201"; got != want {
			t.Fatalf("unexpected escaped path: got=%q want=%q", got, want)
		}
		if got, want := r.Header.Get("X-API-Key"), "control-plane-secret"; got != want {
			t.Fatalf("unexpected api key header: got=%q want=%q", got, want)
		}
		if got, want := r.Header.Get(sharedobservability.RequestIDHeader), "req-123"; got != want {
			t.Fatalf("unexpected request id header: got=%q want=%q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"wallet/hot 1",
			"type":"wallet",
			"name":"wallet hot 1",
			"environment":"prod",
			"chain":"ethereum",
			"labels":{"tier":"hot"},
			"criticality":"high"
		}`))
	}))
	defer server.Close()

	client := NewControlPlaneClient(server.URL, 0).WithAPIKey("control-plane-secret")
	ctx := sharedobservability.ContextWithRequestID(context.Background(), "req-123")
	resource, err := client.GetByID(ctx, "wallet/hot 1")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if resource.ID != "wallet/hot 1" {
		t.Fatalf("unexpected resource id: %q", resource.ID)
	}
}

func TestControlPlaneClientListPoliciesIncludesTrapFlag(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("action_type"), "withdrawal"; got != want {
			t.Fatalf("unexpected action_type query: got=%q want=%q", got, want)
		}
		if got, want := r.URL.Query().Get("resource_type"), "wallet"; got != want {
			t.Fatalf("unexpected resource_type query: got=%q want=%q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items":[
				{
					"id":"policy_canary_trap",
					"action_type":"*",
					"resource_type":"*",
					"effect":"deny",
					"priority":-1000,
					"expression":"resource.labels[\"_nexus_trap\"] == \"true\"",
					"reason":"canary resource should never receive actions",
					"require_approval":false,
					"approval_ttl_seconds":0,
					"is_trap":true,
					"enabled":true
				}
			]
		}`))
	}))
	defer server.Close()

	items, err := NewControlPlaneClient(server.URL, 0).List(context.Background(), "withdrawal", "wallet")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || !items[0].IsTrap {
		t.Fatalf("unexpected policies: %#v", items)
	}
}
