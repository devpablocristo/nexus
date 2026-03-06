package assistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"nexus/pkg/types"
)

func TestQuery_ForwardsTenantScopedPayloadAndParsesResponse(t *testing.T) {
	var seenPath string
	var seenOperatorKey string
	var seenPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenOperatorKey = r.Header.Get("X-Operator-Key")
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&seenPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary":"Tenant posture looks stable.",
			"tables":[{"title":"Operator State","columns":["field","value"],"rows":[{"field":"tenant_status","value":"active"}]}],
			"actions":[{"label":"Run operator tick now","action_type":"operator.tick","payload":{"endpoint":"/v1/internal/tick"}}]
		}`))
	}))
	defer server.Close()

	actor := "alice"
	orgID := uuid.New()
	uc := NewUsecases(Config{
		OperatorBaseURL: server.URL,
		OperatorAPIKey:  "op-key",
	})

	out, err := uc.Query(context.Background(), orgID, &actor, "What changed?")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}

	if seenPath != "/v1/assistant/query" {
		t.Fatalf("unexpected path: %s", seenPath)
	}
	if seenOperatorKey != "op-key" {
		t.Fatalf("unexpected operator key: %s", seenOperatorKey)
	}
	if got := seenPayload["org_id"]; got != orgID.String() {
		t.Fatalf("unexpected org_id: %v", got)
	}
	if got := seenPayload["query"]; got != "What changed?" {
		t.Fatalf("unexpected query: %v", got)
	}
	if got := seenPayload["actor"]; got != actor {
		t.Fatalf("unexpected actor: %v", got)
	}
	if out.Summary != "Tenant posture looks stable." {
		t.Fatalf("unexpected summary: %s", out.Summary)
	}
	if len(out.Tables) != 1 || out.Tables[0].Title != "Operator State" {
		t.Fatalf("unexpected tables: %+v", out.Tables)
	}
	if len(out.Actions) != 1 || out.Actions[0].ActionType != "operator.tick" {
		t.Fatalf("unexpected actions: %+v", out.Actions)
	}
}

func TestQuery_MapsUpstreamFailureToHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	uc := NewUsecases(Config{
		OperatorBaseURL: server.URL,
		OperatorAPIKey:  "op-key",
	})

	_, err := uc.Query(context.Background(), uuid.New(), nil, "status?")
	if err == nil {
		t.Fatal("expected error")
	}
	httpErr, ok := err.(types.HTTPError)
	if !ok {
		t.Fatalf("expected types.HTTPError, got %T", err)
	}
	if httpErr.Status != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", httpErr.Status)
	}
}

func TestTick_UsesInternalTickEndpoint(t *testing.T) {
	var seenPath string
	var seenOperatorKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenOperatorKey = r.Header.Get("X-Operator-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uc := NewUsecases(Config{
		OperatorBaseURL: server.URL,
		OperatorAPIKey:  "op-key",
	})

	if err := uc.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if seenPath != "/v1/internal/tick" {
		t.Fatalf("unexpected path: %s", seenPath)
	}
	if seenOperatorKey != "op-key" {
		t.Fatalf("unexpected operator key: %s", seenOperatorKey)
	}
}
