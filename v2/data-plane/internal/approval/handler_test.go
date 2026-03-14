package approval_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
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

	leaseResp := doJSON[gwdto.ExecutionLeaseItem](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/lease", nil, http.StatusCreated)
	if leaseResp.IntentID != runResp.IntentID {
		t.Fatalf("unexpected lease response: %#v", leaseResp)
	}
	if leaseResp.Status != "active" {
		t.Fatalf("unexpected lease status: %#v", leaseResp)
	}

	executeResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/execute", map[string]any{
		"lease_id": leaseResp.ID,
	}, http.StatusOK)
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

func TestExecuteIntentLeaseIsSingleUse(t *testing.T) {
	t.Parallel()

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		time.Sleep(150 * time.Millisecond)
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

	doJSON[approvaldto.DecideResponse](t, server, http.MethodPost, "/v1/approvals/"+runResp.ApprovalID+"/approve", map[string]any{
		"decided_by": "alice",
	}, http.StatusOK)

	leaseResp := doJSON[gwdto.ExecutionLeaseItem](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/lease", nil, http.StatusCreated)

	type execResult struct {
		status int
		body   []byte
	}

	start := make(chan struct{})
	results := make(chan execResult, 2)
	var wg sync.WaitGroup

	execute := func() {
		defer wg.Done()
		<-start

		payload := bytes.NewBuffer(nil)
		if err := json.NewEncoder(payload).Encode(map[string]any{"lease_id": leaseResp.ID}); err != nil {
			t.Errorf("encode request: %v", err)
			return
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/execute", payload)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		results <- execResult{status: rec.Code, body: rec.Body.Bytes()}
	}

	for range 2 {
		wg.Add(1)
		go execute()
	}

	close(start)
	wg.Wait()
	close(results)

	var successCount int
	var blockedCount int
	for result := range results {
		switch result.status {
		case http.StatusOK:
			var resp gwdto.RunResponse
			if err := json.Unmarshal(result.body, &resp); err != nil {
				t.Fatalf("decode success response: %v", err)
			}
			if resp.Status != "success" || resp.Decision != "allow" {
				t.Fatalf("unexpected execute success response: %#v", resp)
			}
			successCount++
		case http.StatusForbidden:
			var resp gwdto.RunResponse
			if err := json.Unmarshal(result.body, &resp); err != nil {
				t.Fatalf("decode blocked response: %v", err)
			}
			if resp.Status != "blocked" || resp.Reason != "execution lease is not active for this intent" {
				t.Fatalf("unexpected blocked response: %#v", resp)
			}
			blockedCount++
		default:
			t.Fatalf("unexpected execute status: %d body=%s", result.status, string(result.body))
		}
	}

	if successCount != 1 || blockedCount != 1 {
		t.Fatalf("unexpected execute outcomes: success=%d blocked=%d", successCount, blockedCount)
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("expected single upstream call with reused lease, got=%d", got)
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

			executeResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/execute", map[string]any{
				"lease_id": "11111111-1111-1111-1111-111111111111",
			}, tt.wantHTTPCode)
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

func TestIntentPreflightEndpoint(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "deploy"}})
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
		"expression":           `input.hello == "deploy"`,
		"reason":               "operator approval required",
		"require_approval":     true,
		"approval_ttl_seconds": 300,
	}, http.StatusCreated)

	runResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello":         "deploy",
			"environment":   "production",
			"change_ticket": "CHG-123",
		},
	}, http.StatusAccepted)

	preflightResp := doJSON[gwdto.PreflightReviewResponse](t, server, http.MethodGet, "/v1/run/intents/"+runResp.IntentID+"/preflight", nil, http.StatusOK)
	if preflightResp.IntentID != runResp.IntentID {
		t.Fatalf("unexpected preflight intent id: %#v", preflightResp)
	}
	if preflightResp.RiskClass != "mutate_prod" {
		t.Fatalf("unexpected risk class: %#v", preflightResp)
	}
	if preflightResp.Status != "passed" {
		t.Fatalf("unexpected preflight status: %#v", preflightResp)
	}
	if preflightResp.CompletedAt == nil {
		t.Fatalf("expected completed_at in preflight response: %#v", preflightResp)
	}
	if required, ok := preflightResp.Summary["required"].(bool); !ok || !required {
		t.Fatalf("expected preflight required summary: %#v", preflightResp)
	}
}

func TestIssueExecutionLeaseAfterApproval(t *testing.T) {
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

	doJSON[approvaldto.DecideResponse](t, server, http.MethodPost, "/v1/approvals/"+runResp.ApprovalID+"/approve", map[string]any{
		"decided_by": "alice",
	}, http.StatusOK)

	leaseResp := doJSON[gwdto.ExecutionLeaseItem](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/lease", nil, http.StatusCreated)
	if leaseResp.IntentID != runResp.IntentID {
		t.Fatalf("unexpected lease item: %#v", leaseResp)
	}
	if leaseResp.Status != "active" {
		t.Fatalf("unexpected active status: %#v", leaseResp)
	}
	if leaseResp.CredentialMode == "" {
		t.Fatalf("expected credential mode: %#v", leaseResp)
	}
}

func TestIssueExecutionLeaseRequiresApprovedIntent(t *testing.T) {
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

	errResp := doJSON[gwdto.ErrorResponse](t, server, http.MethodPost, "/v1/run/intents/"+runResp.IntentID+"/lease", nil, http.StatusForbidden)
	if errResp.Error.Code != "APPROVAL_REQUIRED" {
		t.Fatalf("unexpected lease error: %#v", errResp)
	}
}

func TestRunBlocksWhenDeterministicPreflightFails(t *testing.T) {
	t.Parallel()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "deploy"}})
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
		"expression":           `input.hello == "deploy"`,
		"reason":               "operator approval required",
		"require_approval":     true,
		"approval_ttl_seconds": 300,
	}, http.StatusCreated)

	runResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello":       "deploy",
			"environment": "production",
		},
	}, http.StatusForbidden)
	if runResp.Status != "blocked" || runResp.Decision != "deny" {
		t.Fatalf("unexpected blocked run response: %#v", runResp)
	}
	if runResp.Reason != "preflight requires change_ticket for production execution" {
		t.Fatalf("unexpected preflight failure reason: %#v", runResp)
	}
	if runResp.IntentID != "" || runResp.ApprovalID != "" {
		t.Fatalf("did not expect intent or approval ids when preflight blocks: %#v", runResp)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream should not be called on failed preflight, got=%d", upstreamCalls)
	}

	intentsResp := doJSON[gwdto.ListIntentsResponse](t, server, http.MethodGet, "/v1/run/intents", nil, http.StatusOK)
	if len(intentsResp.Items) != 0 {
		t.Fatalf("expected no intents after failed preflight, got=%d", len(intentsResp.Items))
	}

	approvalsResp := doJSON[approvaldto.ListApprovalsResponse](t, server, http.MethodGet, "/v1/approvals", nil, http.StatusOK)
	if len(approvalsResp.Items) != 0 {
		t.Fatalf("expected no approvals after failed preflight, got=%d", len(approvalsResp.Items))
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
