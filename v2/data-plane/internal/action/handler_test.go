package action

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	actiondto "nexus/v2/data-plane/internal/action/handler/dto"
)

func TestActionLifecycleEndpoints(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/actions", bytes.NewBufferString(`{
		"action_type":"withdrawal",
		"resource_id":"wallet_hot_usdc_1",
		"resource_type":"wallet",
		"source_system":"treasury-orchestrator",
		"justification":"Daily settlement withdrawal",
		"requested_by":{"type":"system","id":"treasury-bot"},
		"proposed_by":{"type":"agent","id":"treasury-agent"},
		"payload":{
			"asset":"USDC",
			"amount":"25000.00",
			"network":"ethereum",
			"destination_address":"0x123"
		},
		"metadata":{"ticket_id":"CHG-1234"}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created actiondto.ActionResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Status != "pending_approval" || created.Decision != "require_approval" {
		t.Fatalf("unexpected created action: %#v", created)
	}
	if created.Approval == nil || created.Approval.ApprovalID == nil {
		t.Fatalf("expected approval in created action: %#v", created)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/actions?action_type=withdrawal&status=pending_approval", nil)
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	var listed actiondto.ListActionsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", listed)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/actions/"+created.ID, nil)
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getRec.Code, getRec.Body.String())
	}

	riskRec := httptest.NewRecorder()
	riskReq := httptest.NewRequest(http.MethodGet, "/v1/actions/"+created.ID+"/risk", nil)
	mux.ServeHTTP(riskRec, riskReq)
	if riskRec.Code != http.StatusOK {
		t.Fatalf("unexpected risk status: %d body=%s", riskRec.Code, riskRec.Body.String())
	}

	var risk actiondto.RiskResponse
	if err := json.NewDecoder(riskRec.Body).Decode(&risk); err != nil {
		t.Fatalf("decode risk response: %v", err)
	}
	if risk.Level != "medium" || risk.Score != 20 || risk.RecommendedDecision != "enhanced_log" {
		t.Fatalf("unexpected risk response: %#v", risk)
	}

	evidenceRec := httptest.NewRecorder()
	evidenceReq := httptest.NewRequest(http.MethodGet, "/v1/actions/"+created.ID+"/evidence", nil)
	mux.ServeHTTP(evidenceRec, evidenceReq)
	if evidenceRec.Code != http.StatusOK {
		t.Fatalf("unexpected evidence status: %d body=%s", evidenceRec.Code, evidenceRec.Body.String())
	}

	var evidence actiondto.EvidenceListResponse
	if err := json.NewDecoder(evidenceRec.Body).Decode(&evidence); err != nil {
		t.Fatalf("decode evidence response: %v", err)
	}
	if len(evidence.Items) == 0 {
		t.Fatal("expected evidence items")
	}
}

func TestActionEndpointsValidation(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	NewHandler(NewUsecases(NewInMemoryRepository(nil))).Register(mux)

	badCreateReq := httptest.NewRequest(http.MethodPost, "/v1/actions", bytes.NewBufferString(`{"action_type":"withdrawal"}`))
	badCreateReq.Header.Set("Content-Type", "application/json")
	badCreateRec := httptest.NewRecorder()
	mux.ServeHTTP(badCreateRec, badCreateReq)
	if badCreateRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected create validation status: %d body=%s", badCreateRec.Code, badCreateRec.Body.String())
	}

	badListReq := httptest.NewRequest(http.MethodGet, "/v1/actions?limit=0", nil)
	badListRec := httptest.NewRecorder()
	mux.ServeHTTP(badListRec, badListReq)
	if badListRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected list validation status: %d body=%s", badListRec.Code, badListRec.Body.String())
	}

	badIDReq := httptest.NewRequest(http.MethodGet, "/v1/actions/not-a-uuid", nil)
	badIDRec := httptest.NewRecorder()
	mux.ServeHTTP(badIDRec, badIDReq)
	if badIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected invalid id status: %d body=%s", badIDRec.Code, badIDRec.Body.String())
	}

	missingID := uuid.New()
	getMissingReq := httptest.NewRequest(http.MethodGet, "/v1/actions/"+missingID.String(), nil)
	getMissingRec := httptest.NewRecorder()
	mux.ServeHTTP(getMissingRec, getMissingReq)
	if getMissingRec.Code != http.StatusNotFound {
		t.Fatalf("unexpected missing status: %d body=%s", getMissingRec.Code, getMissingRec.Body.String())
	}
}

func TestActionApprovalLeaseExecuteEndpoints(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/actions", bytes.NewBufferString(`{
		"action_type":"withdrawal",
		"resource_id":"wallet_hot_usdc_1",
		"resource_type":"wallet",
		"source_system":"treasury-orchestrator",
		"justification":"Daily settlement withdrawal",
		"requested_by":{"type":"system","id":"treasury-bot"},
		"proposed_by":{"type":"agent","id":"treasury-agent"},
		"payload":{"asset":"USDC","amount":"25000.00","network":"ethereum","destination_address":"0x123"}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created actiondto.ActionResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/v1/actions/"+created.ID+"/approve", bytes.NewBufferString(`{
		"decided_by":{"type":"user","id":"alice"},
		"comment":"approved after treasury review"
	}`))
	approveReq.Header.Set("Content-Type", "application/json")
	approveRec := httptest.NewRecorder()
	mux.ServeHTTP(approveRec, approveReq)
	if approveRec.Code != http.StatusOK {
		t.Fatalf("unexpected approve status: %d body=%s", approveRec.Code, approveRec.Body.String())
	}

	var approved actiondto.ActionResponse
	if err := json.NewDecoder(approveRec.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approve response: %v", err)
	}
	if approved.Status != "approved" || approved.Decision != "allow" {
		t.Fatalf("unexpected approved action: %#v", approved)
	}

	leaseReq := httptest.NewRequest(http.MethodPost, "/v1/actions/"+created.ID+"/lease", nil)
	leaseRec := httptest.NewRecorder()
	mux.ServeHTTP(leaseRec, leaseReq)
	if leaseRec.Code != http.StatusOK {
		t.Fatalf("unexpected lease status: %d body=%s", leaseRec.Code, leaseRec.Body.String())
	}

	var leased actiondto.ActionResponse
	if err := json.NewDecoder(leaseRec.Body).Decode(&leased); err != nil {
		t.Fatalf("decode lease response: %v", err)
	}
	if leased.Status != "leased" || leased.Lease == nil {
		t.Fatalf("unexpected leased action: %#v", leased)
	}

	executeReq := httptest.NewRequest(http.MethodPost, "/v1/actions/"+created.ID+"/execute", bytes.NewBufferString(`{
		"lease_id":"`+leased.Lease.ID+`",
		"executed_by":{"type":"system","id":"wallet-orchestrator"}
	}`))
	executeReq.Header.Set("Content-Type", "application/json")
	executeRec := httptest.NewRecorder()
	mux.ServeHTTP(executeRec, executeReq)
	if executeRec.Code != http.StatusOK {
		t.Fatalf("unexpected execute status: %d body=%s", executeRec.Code, executeRec.Body.String())
	}

	var executed actiondto.ActionResponse
	if err := json.NewDecoder(executeRec.Body).Decode(&executed); err != nil {
		t.Fatalf("decode execute response: %v", err)
	}
	if executed.Status != "executed" || executed.Execution == nil || executed.Execution.Status != "success" {
		t.Fatalf("unexpected executed action: %#v", executed)
	}
	if executed.Lease == nil || executed.Lease.Status != "used" {
		t.Fatalf("expected used lease: %#v", executed.Lease)
	}
}

func TestActionRejectAndExecuteValidation(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	mux := http.NewServeMux()
	NewHandler(uc).Register(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/actions", bytes.NewBufferString(`{
		"action_type":"withdrawal",
		"resource_id":"wallet_hot_usdc_1",
		"resource_type":"wallet",
		"source_system":"treasury-orchestrator",
		"justification":"Daily settlement withdrawal",
		"requested_by":{"type":"system","id":"treasury-bot"},
		"proposed_by":{"type":"agent","id":"treasury-agent"},
		"payload":{"asset":"USDC","amount":"25000.00","network":"ethereum","destination_address":"0x123"}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	var created actiondto.ActionResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	rejectReq := httptest.NewRequest(http.MethodPost, "/v1/actions/"+created.ID+"/reject", bytes.NewBufferString(`{
		"decided_by":{"type":"user","id":"alice"},
		"comment":"destination not approved"
	}`))
	rejectReq.Header.Set("Content-Type", "application/json")
	rejectRec := httptest.NewRecorder()
	mux.ServeHTTP(rejectRec, rejectReq)
	if rejectRec.Code != http.StatusOK {
		t.Fatalf("unexpected reject status: %d body=%s", rejectRec.Code, rejectRec.Body.String())
	}

	leaseReq := httptest.NewRequest(http.MethodPost, "/v1/actions/"+created.ID+"/lease", nil)
	leaseRec := httptest.NewRecorder()
	mux.ServeHTTP(leaseRec, leaseReq)
	if leaseRec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden lease status: %d body=%s", leaseRec.Code, leaseRec.Body.String())
	}

	badExecuteReq := httptest.NewRequest(http.MethodPost, "/v1/actions/"+created.ID+"/execute", bytes.NewBufferString(`{
		"lease_id":"not-a-uuid",
		"executed_by":{"type":"system","id":"wallet-orchestrator"}
	}`))
	badExecuteReq.Header.Set("Content-Type", "application/json")
	badExecuteRec := httptest.NewRecorder()
	mux.ServeHTTP(badExecuteRec, badExecuteReq)
	if badExecuteRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected invalid lease status: %d body=%s", badExecuteRec.Code, badExecuteRec.Body.String())
	}
}
