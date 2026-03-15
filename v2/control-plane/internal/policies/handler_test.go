package policies

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	policydto "nexus/v2/control-plane/internal/policies/handler/dto"
)

func TestPolicyEndpointsLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil), NewEvaluator())
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/policies", bytes.NewBufferString(`{
		"action_type":"withdrawal",
		"resource_type":"wallet",
		"effect":"allow",
		"priority":10,
		"expression":"action.action_type == \"withdrawal\" && resource.type == \"wallet\"",
		"reason":"withdrawals from wallets require approval",
		"require_approval":true,
		"approval_ttl_seconds":600
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created policydto.PolicyResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.ActionType != "withdrawal" {
		t.Fatalf("unexpected created policy: %#v", created)
	}
	if got := createRec.Header().Get("Location"); got != "/v1/policies/"+created.ID {
		t.Fatalf("unexpected location header: %q", got)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/policies?action_type=withdrawal&resource_type=wallet", nil)
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	archiveRec := httptest.NewRecorder()
	archiveReq := httptest.NewRequest(http.MethodPost, "/v1/policies/"+created.ID+"/archive", nil)
	mux.ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("unexpected archive status: %d body=%s", archiveRec.Code, archiveRec.Body.String())
	}

	archivedListRec := httptest.NewRecorder()
	archivedListReq := httptest.NewRequest(http.MethodGet, "/v1/policies?archived=true", nil)
	mux.ServeHTTP(archivedListRec, archivedListReq)
	if archivedListRec.Code != http.StatusOK {
		t.Fatalf("unexpected archived list status: %d body=%s", archivedListRec.Code, archivedListRec.Body.String())
	}

	var archivedList policydto.ListPoliciesResponse
	if err := json.NewDecoder(archivedListRec.Body).Decode(&archivedList); err != nil {
		t.Fatalf("decode archived list response: %v", err)
	}
	if len(archivedList.Items) != 1 || !archivedList.Items[0].Archived {
		t.Fatalf("unexpected archived list: %#v", archivedList)
	}
}
