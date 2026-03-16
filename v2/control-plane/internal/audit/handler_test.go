package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auditdto "nexus/v2/control-plane/internal/audit/handler/dto"
)

func TestAuditEndpoints(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/internal/audit", bytes.NewBufferString(`{
		"event_type":"action_created",
		"source_service":"data-plane",
		"action_id":"action-1",
		"incident_id":"incident-1",
		"alert_id":"alert-1",
		"resource_id":"resource-1",
		"resource_type":"wallet",
		"actor":{"type":"system","id":"treasury-bot"},
		"summary":"withdrawal created",
		"data":{"status":"pending_approval"}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created auditdto.AuditResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.EventType != "action_created" {
		t.Fatalf("unexpected created audit record: %#v", created)
	}
	if created.IncidentID != "incident-1" || created.AlertID != "alert-1" {
		t.Fatalf("unexpected created correlation fields: %#v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/audit?action_id=action-1&incident_id=incident-1&alert_id=alert-1&actor_id=treasury-bot&event_type=action_created&from="+created.OccurredAt.Add(-time.Minute).Format(time.RFC3339)+"&to="+created.OccurredAt.Add(time.Minute).Format(time.RFC3339), nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	var listed auditdto.ListAuditResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", listed)
	}
	if listed.Items[0].IncidentID != "incident-1" || listed.Items[0].AlertID != "alert-1" {
		t.Fatalf("unexpected list correlation fields: %#v", listed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/audit/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestAuditEndpointsValidation(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	badCreateReq := httptest.NewRequest(http.MethodPost, "/internal/audit", bytes.NewBufferString(`{"source_service":"data-plane"}`))
	badCreateReq.Header.Set("Content-Type", "application/json")
	badCreateRec := httptest.NewRecorder()
	mux.ServeHTTP(badCreateRec, badCreateReq)
	if badCreateRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad create status: %d body=%s", badCreateRec.Code, badCreateRec.Body.String())
	}

	badListReq := httptest.NewRequest(http.MethodGet, "/v1/audit?from=bad-time", nil)
	badListRec := httptest.NewRecorder()
	mux.ServeHTTP(badListRec, badListReq)
	if badListRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad list status: %d body=%s", badListRec.Code, badListRec.Body.String())
	}
}
