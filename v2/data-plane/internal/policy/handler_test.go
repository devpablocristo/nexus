package policy_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gwdto "nexus/v2/data-plane/internal/gateway/handler/dto"
	policydto "nexus/v2/data-plane/internal/policy/handler/dto"
	"nexus/v2/data-plane/wire"
)

func TestPolicyLifecycle(t *testing.T) {
	t.Parallel()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"received": map[string]any{
				"hello": "ok",
			},
		})
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

	createResp := doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":  "echo",
		"effect":     "deny",
		"expression": `input.hello == "blocked"`,
		"reason":     "blocked by policy",
	}, http.StatusCreated)

	if createResp.ToolName != "echo" {
		t.Fatalf("unexpected tool_name: %s", createResp.ToolName)
	}
	if createResp.ID == "" {
		t.Fatal("expected policy id")
	}
	if createResp.Archived {
		t.Fatal("created policy should not be archived")
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/policies", bytes.NewBufferString(`{
		"tool_name":"echo",
		"effect":"deny",
		"expression":"input.hello == \"header-check\""
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	server.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected location-check status: %d body=%s", createRec.Code, createRec.Body.String())
	}
	location := strings.TrimSpace(createRec.Header().Get("Location"))
	if location == "" || !strings.HasPrefix(location, "/v1/policies/") {
		t.Fatalf("unexpected location header: %q", location)
	}

	listResp := doJSON[policydto.ListPoliciesResponse](t, server, http.MethodGet, "/v1/policies", nil, http.StatusOK)
	if len(listResp.Items) != 2 {
		t.Fatalf("unexpected list size: %d", len(listResp.Items))
	}

	getResp := doJSON[policydto.PolicyResponse](t, server, http.MethodGet, "/v1/policies/"+createResp.ID, nil, http.StatusOK)
	if getResp.ID != createResp.ID {
		t.Fatalf("unexpected get id: %s", getResp.ID)
	}

	blockedRun := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello": "blocked",
		},
	}, http.StatusForbidden)
	if blockedRun.Status != "blocked" || blockedRun.Decision != "deny" {
		t.Fatalf("unexpected blocked run: %#v", blockedRun)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream should not be called when policy blocks, got=%d", upstreamCalls)
	}

	patchResp := doJSON[policydto.PolicyResponse](t, server, http.MethodPatch, "/v1/policies/"+createResp.ID, map[string]any{
		"expression": `input.hello == "patched"`,
		"reason":     "patched by policy",
	}, http.StatusOK)
	if patchResp.Expression != `input.hello == "patched"` {
		t.Fatalf("unexpected patched expression: %s", patchResp.Expression)
	}

	allowedRun := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello": "blocked",
		},
	}, http.StatusOK)
	if allowedRun.Status != "success" || allowedRun.Decision != "allow" {
		t.Fatalf("unexpected allowed run: %#v", allowedRun)
	}
	if upstreamCalls != 1 {
		t.Fatalf("expected one upstream call after patch, got=%d", upstreamCalls)
	}

	archiveResp := doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies/"+createResp.ID+"/archive", map[string]any{}, http.StatusOK)
	if !archiveResp.Archived {
		t.Fatal("expected archived policy")
	}

	alreadyArchivedResp := doJSON[policydto.ErrorResponse](t, server, http.MethodPost, "/v1/policies/"+createResp.ID+"/archive", map[string]any{}, http.StatusConflict)
	if alreadyArchivedResp.Error.Code != "ALREADY_ARCHIVED" {
		t.Fatalf("unexpected already archived error code: %s", alreadyArchivedResp.Error.Code)
	}

	archivedPatchResp := doJSON[policydto.ErrorResponse](t, server, http.MethodPatch, "/v1/policies/"+createResp.ID, map[string]any{
		"reason": "should fail while archived",
	}, http.StatusConflict)
	if archivedPatchResp.Error.Code != "ARCHIVED" {
		t.Fatalf("unexpected archived patch error code: %s", archivedPatchResp.Error.Code)
	}

	listAfterArchive := doJSON[policydto.ListPoliciesResponse](t, server, http.MethodGet, "/v1/policies", nil, http.StatusOK)
	if len(listAfterArchive.Items) != 1 {
		t.Fatalf("expected one visible policy after archive, got=%d", len(listAfterArchive.Items))
	}

	listIncludingArchived := doJSON[policydto.ListPoliciesResponse](t, server, http.MethodGet, "/v1/policies?archived=true", nil, http.StatusOK)
	if len(listIncludingArchived.Items) != 1 || !listIncludingArchived.Items[0].Archived {
		t.Fatalf("expected archived item in archived list: %#v", listIncludingArchived.Items)
	}

	restoreResp := doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies/"+createResp.ID+"/restore", map[string]any{}, http.StatusOK)
	if restoreResp.Archived {
		t.Fatal("expected restored policy")
	}

	notArchivedResp := doJSON[policydto.ErrorResponse](t, server, http.MethodPost, "/v1/policies/"+createResp.ID+"/restore", map[string]any{}, http.StatusConflict)
	if notArchivedResp.Error.Code != "NOT_ARCHIVED" {
		t.Fatalf("unexpected not archived error code: %s", notArchivedResp.Error.Code)
	}

	reblockedRun := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello": "patched",
		},
	}, http.StatusForbidden)
	if reblockedRun.Status != "blocked" || reblockedRun.Decision != "deny" {
		t.Fatalf("unexpected reblocked run: %#v", reblockedRun)
	}

	doNoContent(t, server, http.MethodDelete, "/v1/policies/"+createResp.ID, http.StatusNoContent)

	notFoundResp := doJSON[policydto.ErrorResponse](t, server, http.MethodGet, "/v1/policies/"+createResp.ID, nil, http.StatusNotFound)
	if notFoundResp.Error.Code != "NOT_FOUND" {
		t.Fatalf("unexpected error code after delete: %s", notFoundResp.Error.Code)
	}
}

func TestPolicyCreateRejectsInvalidExpression(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{}})
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

	resp := doJSON[policydto.ErrorResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":  "echo",
		"effect":     "deny",
		"expression": `input.hello ==`,
	}, http.StatusBadRequest)

	if resp.Error.Code != "INVALID_EXPRESSION" {
		t.Fatalf("unexpected error code: %s", resp.Error.Code)
	}
}

func TestPolicyCreateRejectsNonBoolExpression(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{}})
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

	resp := doJSON[policydto.ErrorResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":  "echo",
		"effect":     "deny",
		"expression": `"blocked"`,
	}, http.StatusBadRequest)

	if resp.Error.Code != "INVALID_EXPRESSION" {
		t.Fatalf("unexpected error code: %s", resp.Error.Code)
	}
}

func TestPolicyApprovalLifecycle(t *testing.T) {
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

	createResp := doJSON[policydto.PolicyResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":            "echo",
		"effect":               "allow",
		"expression":           `input.hello == "review"`,
		"reason":               "operator approval required",
		"require_approval":     true,
		"approval_ttl_seconds": 120,
	}, http.StatusCreated)

	if !createResp.RequireApproval || createResp.ApprovalTTLSeconds != 120 {
		t.Fatalf("unexpected create response: %#v", createResp)
	}

	runResp := doJSON[gwdto.RunResponse](t, server, http.MethodPost, "/v1/run", map[string]any{
		"tool_name": "echo",
		"input": map[string]any{
			"hello": "review",
		},
	}, http.StatusAccepted)

	if runResp.IntentID == "" || runResp.ApprovalID == "" {
		t.Fatalf("expected intent_id and approval_id: %#v", runResp)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream should not be called while approval is pending, got=%d", upstreamCalls)
	}
}

func TestPolicyCreateRejectsApprovalOnDeny(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{}})
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

	resp := doJSON[policydto.ErrorResponse](t, server, http.MethodPost, "/v1/policies", map[string]any{
		"tool_name":        "echo",
		"effect":           "deny",
		"expression":       `input.hello == "blocked"`,
		"require_approval": true,
	}, http.StatusBadRequest)

	if resp.Error.Code != "VALIDATION" {
		t.Fatalf("unexpected error code: %s", resp.Error.Code)
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
	if wantStatus == http.StatusNoContent {
		return out
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func doNoContent(t *testing.T, handler http.Handler, method, path string, wantStatus int) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("unexpected status: got=%d want=%d body=%s", rec.Code, wantStatus, rec.Body.String())
	}
}
