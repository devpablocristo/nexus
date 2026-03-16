package incidents

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	incidentdto "nexus/v2/control-workers/internal/incidents/handler/dto"
)

func TestIncidentEndpointsLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/incidents", bytes.NewBufferString(`{
		"source_kind":"action",
		"source_id":"action-1",
		"action_type":"withdrawal",
		"resource_id":"wallet_hot_usdc_1",
		"resource_type":"wallet",
		"trigger":"execution_failed",
		"risk_level":"critical",
		"reason":"executor could not reach signer",
		"details":{"attempt":1}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created incidentdto.IncidentResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Status != "open" || created.Severity != "critical" {
		t.Fatalf("unexpected created incident: %#v", created)
	}
	if created.ActionID != "action-1" {
		t.Fatalf("unexpected created incident correlation fields: %#v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/incidents?trigger=execution_failed&status=open", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	var listed incidentdto.ListIncidentsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", listed)
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/v1/incidents/"+created.ID, bytes.NewBufferString(`{
		"status":"resolved",
		"summary":"withdrawal incident resolved after manual review"
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	archiveReq := httptest.NewRequest(http.MethodPost, "/v1/incidents/"+created.ID+"/archive", nil)
	archiveRec := httptest.NewRecorder()
	mux.ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("unexpected archive status: %d body=%s", archiveRec.Code, archiveRec.Body.String())
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/v1/incidents/"+created.ID+"/restore", nil)
	restoreRec := httptest.NewRecorder()
	mux.ServeHTTP(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("unexpected restore status: %d body=%s", restoreRec.Code, restoreRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/incidents/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getRec.Code, getRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/incidents/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete status: %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestIncidentEndpointsValidation(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	badCreateReq := httptest.NewRequest(http.MethodPost, "/v1/incidents", bytes.NewBufferString(`{"source_kind":"action"}`))
	badCreateReq.Header.Set("Content-Type", "application/json")
	badCreateRec := httptest.NewRecorder()
	mux.ServeHTTP(badCreateRec, badCreateReq)
	if badCreateRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad create status: %d body=%s", badCreateRec.Code, badCreateRec.Body.String())
	}

	badListReq := httptest.NewRequest(http.MethodGet, "/v1/incidents?limit=0", nil)
	badListRec := httptest.NewRecorder()
	mux.ServeHTTP(badListRec, badListReq)
	if badListRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad list status: %d body=%s", badListRec.Code, badListRec.Body.String())
	}

	badIDReq := httptest.NewRequest(http.MethodGet, "/v1/incidents/not-a-uuid", nil)
	badIDRec := httptest.NewRecorder()
	mux.ServeHTTP(badIDRec, badIDReq)
	if badIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad id status: %d body=%s", badIDRec.Code, badIDRec.Body.String())
	}
}
