package alerts

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	alertdto "nexus/v2/control-workers/internal/alerts/handler/dto"
)

func TestAlertEndpointsLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/alerts", bytes.NewBufferString(`{
		"source_kind":"incident",
		"source_id":"incident-1",
		"action_id":"action-1",
		"resource_id":"wallet_hot_usdc_1",
		"resource_type":"wallet",
		"channel":"slack",
		"route":"ops-p2",
		"severity":"high",
		"summary":"withdrawal blocked by Nexus",
		"body":"incident requires operator attention",
		"details":{"incident_id":"incident-1"}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created alertdto.AlertResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Status != "pending" {
		t.Fatalf("unexpected created alert: %#v", created)
	}
	if created.IncidentID != "incident-1" || created.ActionID != "action-1" || created.ResourceID != "wallet_hot_usdc_1" || created.ResourceType != "wallet" {
		t.Fatalf("unexpected created alert correlation fields: %#v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/alerts?channel=slack&status=pending", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/v1/alerts/"+created.ID, bytes.NewBufferString(`{
		"status":"acknowledged",
		"summary":"alert acknowledged by operator"
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	archiveReq := httptest.NewRequest(http.MethodPost, "/v1/alerts/"+created.ID+"/archive", nil)
	archiveRec := httptest.NewRecorder()
	mux.ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("unexpected archive status: %d body=%s", archiveRec.Code, archiveRec.Body.String())
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/v1/alerts/"+created.ID+"/restore", nil)
	restoreRec := httptest.NewRecorder()
	mux.ServeHTTP(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("unexpected restore status: %d body=%s", restoreRec.Code, restoreRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/alerts/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getRec.Code, getRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/alerts/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete status: %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestAlertEndpointsValidation(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	badCreateReq := httptest.NewRequest(http.MethodPost, "/v1/alerts", bytes.NewBufferString(`{"source_kind":"incident"}`))
	badCreateReq.Header.Set("Content-Type", "application/json")
	badCreateRec := httptest.NewRecorder()
	mux.ServeHTTP(badCreateRec, badCreateReq)
	if badCreateRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad create status: %d body=%s", badCreateRec.Code, badCreateRec.Body.String())
	}

	badListReq := httptest.NewRequest(http.MethodGet, "/v1/alerts?limit=0", nil)
	badListRec := httptest.NewRecorder()
	mux.ServeHTTP(badListRec, badListReq)
	if badListRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad list status: %d body=%s", badListRec.Code, badListRec.Body.String())
	}

	badIDReq := httptest.NewRequest(http.MethodGet, "/v1/alerts/not-a-uuid", nil)
	badIDRec := httptest.NewRecorder()
	mux.ServeHTTP(badIDRec, badIDReq)
	if badIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad id status: %d body=%s", badIDRec.Code, badIDRec.Body.String())
	}
}
