package approval_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	approvaldto "nexus/v2/data-plane/internal/approval/handler/dto"
	gwdto "nexus/v2/data-plane/internal/gateway/handler/dto"
	policydto "nexus/v2/data-plane/internal/policy/handler/dto"
	"nexus/v2/data-plane/wire"
)

func TestApprovalLifecycle(t *testing.T) {
	t.Parallel()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "review"}})
	}))
	defer upstream.Close()

	server, cleanup, err := wire.NewServer(wire.Config{
		EchoURL:     upstream.URL,
		HTTPTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("wire.NewServer returned error: %v", err)
	}
	defer cleanup()

	createPolicy := doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":            "echo",
		"effect":               "allow",
		"expression":           `input.hello == "review"`,
		"reason":               "operator approval required",
		"require_approval":     true,
		"approval_ttl_seconds": 300,
	}, http.StatusCreated)
	if !createPolicy.RequireApproval {
		t.Fatalf("expected require_approval=true: %#v", createPolicy)
	}

	runResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello": "review",
		},
	}, http.StatusAccepted)
	if runResp.ApprovalID == "" {
		t.Fatalf("expected approval id: %#v", runResp)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream should not be called while approval pending, got=%d", upstreamCalls)
	}

	listResp := doJSON[approvaldto.ListApprovalsResponse](t, server, http.MethodGet, "/v1/approvals", nil, http.StatusOK)
	if len(listResp.Items) != 1 {
		t.Fatalf("unexpected pending approvals count: %d", len(listResp.Items))
	}
	if listResp.Items[0].ID != runResp.ApprovalID {
		t.Fatalf("unexpected approval id in list: %#v", listResp.Items[0])
	}
	if listResp.Items[0].Status != "pending" {
		t.Fatalf("unexpected pending status: %#v", listResp.Items[0])
	}

	getResp := doJSON[approvaldto.ApprovalItem](t, server, http.MethodGet, "/v1/approvals/"+runResp.ApprovalID, nil, http.StatusOK)
	if getResp.IntentID == nil || *getResp.IntentID != runResp.IntentID {
		t.Fatalf("unexpected intent link: %#v", getResp)
	}

	approveResp := doJSON[approvaldto.DecideResponse](t, server, http.MethodPost, "/v1/approvals/"+runResp.ApprovalID+"/approve", map[string]any{
		"decided_by": "alice",
	}, http.StatusOK)
	if approveResp.Status != "approved" {
		t.Fatalf("unexpected approve response: %#v", approveResp)
	}

	getApproved := doJSON[approvaldto.ApprovalItem](t, server, http.MethodGet, "/v1/approvals/"+runResp.ApprovalID, nil, http.StatusOK)
	if getApproved.Status != "approved" {
		t.Fatalf("unexpected approved item: %#v", getApproved)
	}
	if getApproved.DecidedBy == nil || *getApproved.DecidedBy != "alice" {
		t.Fatalf("unexpected decided_by: %#v", getApproved)
	}

	emptyList := doJSON[approvaldto.ListApprovalsResponse](t, server, http.MethodGet, "/v1/approvals", nil, http.StatusOK)
	if len(emptyList.Items) != 0 {
		t.Fatalf("expected no pending approvals after approval, got=%d", len(emptyList.Items))
	}
}

func TestApprovalRejectAndAlreadyDecided(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "review"}})
	}))
	defer upstream.Close()

	server, cleanup, err := wire.NewServer(wire.Config{
		EchoURL:     upstream.URL,
		HTTPTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("wire.NewServer returned error: %v", err)
	}
	defer cleanup()

	doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":            "echo",
		"effect":               "allow",
		"expression":           `input.hello == "review"`,
		"reason":               "operator approval required",
		"require_approval":     true,
		"approval_ttl_seconds": 300,
	}, http.StatusCreated)

	runResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello": "review",
		},
	}, http.StatusAccepted)

	rejectResp := doJSON[approvaldto.DecideResponse](t, server, http.MethodPost, "/v1/approvals/"+runResp.ApprovalID+"/reject", map[string]any{
		"decided_by": "bob",
	}, http.StatusOK)
	if rejectResp.Status != "rejected" {
		t.Fatalf("unexpected reject response: %#v", rejectResp)
	}

	conflict := doJSON[approvaldto.ErrorResponse](t, server, http.MethodPost, "/v1/approvals/"+runResp.ApprovalID+"/approve", map[string]any{
		"decided_by": "carol",
	}, http.StatusConflict)
	if conflict.Error.Code != "ALREADY_DECIDED" {
		t.Fatalf("unexpected conflict code: %#v", conflict)
	}
}

func TestExecuteIntentAfterApproval(t *testing.T) {
	t.Parallel()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "review"}})
	}))
	defer upstream.Close()

	server, cleanup, err := wire.NewServer(wire.Config{
		EchoURL:     upstream.URL,
		HTTPTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("wire.NewServer returned error: %v", err)
	}
	defer cleanup()

	doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":            "echo",
		"effect":               "allow",
		"expression":           `input.hello == "review"`,
		"reason":               "operator approval required",
		"require_approval":     true,
		"approval_ttl_seconds": 300,
	}, http.StatusCreated)

	runResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello": "review",
		},
	}, http.StatusAccepted)
	if runResp.IntentID == "" || runResp.ApprovalID == "" {
		t.Fatalf("expected intent and approval ids: %#v", runResp)
	}

	doJSON[approvaldto.DecideResponse](t, server, http.MethodPost, "/v1/approvals/"+runResp.ApprovalID+"/approve", map[string]any{
		"decided_by": "alice",
	}, http.StatusOK)

	executeResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/execute", nil, http.StatusOK)
	if executeResp.Status != "success" || executeResp.Decision != "allow" {
		t.Fatalf("unexpected execute response: %#v", executeResp)
	}
	if executeResp.IntentID != runResp.IntentID || executeResp.ApprovalID != runResp.ApprovalID {
		t.Fatalf("unexpected execute linkage: %#v", executeResp)
	}
	if upstreamCalls != 1 {
		t.Fatalf("expected one upstream call after execute, got=%d", upstreamCalls)
	}

	intentResp := doJSON[gwdto.IntentItem](t, server, http.MethodGet, "/v1/run/intents/"+runResp.IntentID, nil, http.StatusOK)
	if intentResp.Status != "executed" {
		t.Fatalf("unexpected intent status after execute: %#v", intentResp)
	}
	if intentResp.ExecutedAt == nil {
		t.Fatalf("expected executed_at after execute: %#v", intentResp)
	}
}

func TestExecuteIntentRequiresApprovedStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		decidePath   string
		decideBody   map[string]any
		wantHTTPCode int
	}{
		{
			name:         "pending approval blocks execute",
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "rejected approval blocks execute",
			decidePath:   "/reject",
			decideBody:   map[string]any{"decided_by": "bob"},
			wantHTTPCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			upstreamCalls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamCalls++
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "review"}})
			}))
			defer upstream.Close()

			server, cleanup, err := wire.NewServer(wire.Config{
				EchoURL:     upstream.URL,
				HTTPTimeout: 2 * time.Second,
			})
			if err != nil {
				t.Fatalf("wire.NewServer returned error: %v", err)
			}
			defer cleanup()

			doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
				"tool_name":            "echo",
				"effect":               "allow",
				"expression":           `input.hello == "review"`,
				"reason":               "operator approval required",
				"require_approval":     true,
				"approval_ttl_seconds": 300,
			}, http.StatusCreated)

			runResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
				"tool_name": "echo",
				"input": map[string]any{
					"hello": "review",
				},
			}, http.StatusAccepted)

			if tt.decidePath != "" {
				doJSON[approvaldto.DecideResponse](t, server, http.MethodPost, "/v1/approvals/"+runResp.ApprovalID+tt.decidePath, tt.decideBody, http.StatusOK)
			}

			executeResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/execute", nil, tt.wantHTTPCode)
			if executeResp.Status != "blocked" || executeResp.Decision != "deny" {
				t.Fatalf("unexpected blocked execute response: %#v", executeResp)
			}
			if executeResp.Reason != "intent is not approved for execution" {
				t.Fatalf("unexpected blocked reason: %#v", executeResp)
			}
			if upstreamCalls != 0 {
				t.Fatalf("upstream should not be called when execute is blocked, got=%d", upstreamCalls)
			}
		})
	}
}

func doJSON[T any](t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) T {
	t.Helper()

	var payload *bytes.Buffer
	if body == nil {
		payload = bytes.NewBuffer(nil)
	} else {
		payload = bytes.NewBuffer(nil)
		if err := json.NewEncoder(payload).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != wantStatus {
		t.Fatalf("unexpected status: got=%d want=%d body=%s", rec.Code, wantStatus, rec.Body.String())
	}

	var out T
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}
