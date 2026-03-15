package resources

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	resourcedto "nexus/v2/control-plane/internal/resources/handler/dto"
)

func TestResourceEndpointsLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/resources", bytes.NewBufferString(`{
		"type":"wallet",
		"name":"wallet hot usdc 1",
		"environment":"prod",
		"chain":"ethereum",
		"labels":{"tier":"hot"},
		"criticality":"critical"
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created resourcedto.ResourceResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Type != "wallet" {
		t.Fatalf("unexpected created resource: %#v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/resources?type=wallet&environment=prod&archived=false", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	var listed resourcedto.ListResourcesResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", listed)
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/v1/resources/"+created.ID, bytes.NewBufferString(`{
		"name":"wallet hot usdc primary",
		"chain":"base"
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	archiveReq := httptest.NewRequest(http.MethodPost, "/v1/resources/"+created.ID+"/archive", nil)
	archiveRec := httptest.NewRecorder()
	mux.ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("unexpected archive status: %d body=%s", archiveRec.Code, archiveRec.Body.String())
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/v1/resources/"+created.ID+"/restore", nil)
	restoreRec := httptest.NewRecorder()
	mux.ServeHTTP(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("unexpected restore status: %d body=%s", restoreRec.Code, restoreRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/resources/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getRec.Code, getRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/resources/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete status: %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestResourceEndpointsValidation(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	badCreateReq := httptest.NewRequest(http.MethodPost, "/v1/resources", bytes.NewBufferString(`{"type":"wallet"}`))
	badCreateReq.Header.Set("Content-Type", "application/json")
	badCreateRec := httptest.NewRecorder()
	mux.ServeHTTP(badCreateRec, badCreateReq)
	if badCreateRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad create status: %d body=%s", badCreateRec.Code, badCreateRec.Body.String())
	}

	badListReq := httptest.NewRequest(http.MethodGet, "/v1/resources?limit=0", nil)
	badListRec := httptest.NewRecorder()
	mux.ServeHTTP(badListRec, badListReq)
	if badListRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad list status: %d body=%s", badListRec.Code, badListRec.Body.String())
	}

	badIDReq := httptest.NewRequest(http.MethodGet, "/v1/resources/not-a-uuid", nil)
	badIDRec := httptest.NewRecorder()
	mux.ServeHTTP(badIDRec, badIDReq)
	if badIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad id status: %d body=%s", badIDRec.Code, badIDRec.Body.String())
	}
}
