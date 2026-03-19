package requests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/nexus/v3/review/internal/approvals"
	approvaldomain "github.com/devpablocristo/nexus/v3/review/internal/approvals/usecases/domain"
	auditdomain "github.com/devpablocristo/nexus/v3/review/internal/audit/usecases/domain"
	"github.com/devpablocristo/nexus/v3/review/internal/requests"
	requestdto "github.com/devpablocristo/nexus/v3/review/internal/requests/handler/dto"
	requestdomain "github.com/devpablocristo/nexus/v3/review/internal/requests/usecases/domain"
)

// --- Fakes para tests ---

type fakeRequestRepo struct {
	mu   sync.RWMutex
	byID map[uuid.UUID]requestdomain.Request
}

func newFakeRequestRepo() *fakeRequestRepo {
	return &fakeRequestRepo{byID: make(map[uuid.UUID]requestdomain.Request)}
}

func (r *fakeRequestRepo) Create(_ context.Context, req requestdomain.Request) (requestdomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	r.byID[req.ID] = req
	return req, nil
}

func (r *fakeRequestRepo) GetByID(_ context.Context, id uuid.UUID) (requestdomain.Request, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	req, ok := r.byID[id]
	if !ok {
		return requestdomain.Request{}, requests.ErrNotFound
	}
	return req, nil
}

func (r *fakeRequestRepo) GetByIdempotencyKey(_ context.Context, _ string) (*requestdomain.Request, error) {
	return nil, nil
}

func (r *fakeRequestRepo) List(_ context.Context, status, actionType string, limit int) ([]requestdomain.Request, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []requestdomain.Request
	for _, req := range r.byID {
		if status != "" && string(req.Status) != status {
			continue
		}
		if actionType != "" && req.ActionType != actionType {
			continue
		}
		out = append(out, req)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRequestRepo) Update(_ context.Context, req requestdomain.Request) (requestdomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[req.ID] = req
	return req, nil
}

type fakeApprovalRepo struct {
	mu   sync.RWMutex
	byID map[uuid.UUID]approvaldomain.Approval
}

func newFakeApprovalRepo() *fakeApprovalRepo {
	return &fakeApprovalRepo{byID: make(map[uuid.UUID]approvaldomain.Approval)}
}

func (r *fakeApprovalRepo) Create(_ context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	r.byID[a.ID] = a
	return a, nil
}

func (r *fakeApprovalRepo) GetByID(_ context.Context, id uuid.UUID) (approvaldomain.Approval, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.byID[id]
	if !ok {
		return approvaldomain.Approval{}, approvals.ErrNotFound
	}
	return a, nil
}

func (r *fakeApprovalRepo) GetByRequestID(_ context.Context, _ uuid.UUID) (*approvaldomain.Approval, error) {
	return nil, nil
}

func (r *fakeApprovalRepo) ListPending(_ context.Context, _ int) ([]approvaldomain.Approval, error) {
	return nil, nil
}

func (r *fakeApprovalRepo) Update(_ context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[a.ID] = a
	return a, nil
}

type fakeAuditRepo struct{}

func (r *fakeAuditRepo) Append(_ context.Context, _ auditdomain.RequestEvent) error { return nil }
func (r *fakeAuditRepo) ListByRequestID(_ context.Context, _ uuid.UUID) ([]auditdomain.RequestEvent, error) {
	return nil, nil
}

type fakePolicyLister struct {
	policies []requests.PolicyForEval
}

func (e *fakePolicyLister) ListActive(_ context.Context) ([]requests.PolicyForEval, error) {
	return e.policies, nil
}

type fakeIdemStore struct {
	mu   sync.RWMutex
	keys map[string]struct {
		id   uuid.UUID
		resp map[string]any
	}
}

func newFakeIdemStore() *fakeIdemStore {
	return &fakeIdemStore{keys: make(map[string]struct {
		id   uuid.UUID
		resp map[string]any
	})}
}

func (s *fakeIdemStore) Get(_ context.Context, key string) (uuid.UUID, map[string]any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.keys[key]
	if !ok {
		return uuid.Nil, nil, false
	}
	return e.id, e.resp, true
}

func (s *fakeIdemStore) Set(_ context.Context, key string, id uuid.UUID, resp map[string]any, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[key] = struct {
		id   uuid.UUID
		resp map[string]any
	}{id, resp}
	return nil
}

// --- Helpers ---

// setupRequestMux crea un mux con fakes y sin políticas activas (default risk → allow para low).
func setupRequestMux() *http.ServeMux {
	return setupRequestMuxWithPolicies(nil)
}

// setupRequestMuxWithPolicies crea un mux con las políticas indicadas.
func setupRequestMuxWithPolicies(policies []requests.PolicyForEval) *http.ServeMux {
	reqRepo := newFakeRequestRepo()
	auditSink := requests.NewAuditSinkAdapter(&fakeAuditRepo{})
	evaluator := requests.NewPolicyEvaluator()
	riskConfig := requests.DefaultRiskConfig()
	ai := requests.NewStubContextualizer()

	uc := requests.NewUsecases(reqRepo, &fakePolicyLister{policies: policies}, newFakeApprovalRepo(), evaluator,
		requests.WithIdempotencyStore(newFakeIdemStore()),
		requests.WithAuditSink(auditSink),
		requests.WithRiskConfig(riskConfig),
		requests.WithAI(ai),
		requests.WithApprovalTTL(time.Hour),
	)
	mux := http.NewServeMux()
	requests.NewHandler(uc).Register(mux)
	return mux
}

func doReq(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, r))
	return rec
}

func decJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v, body: %s", err, rec.Body.String())
	}
}

// --- Tests de Submit ---

func TestSubmitRequestAllowed(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	rec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"ops-bot","action_type":"alert.escalate"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var resp requestdto.SubmitResponse
	decJSON(t, rec, &resp)
	if resp.Decision != "allow" {
		t.Fatalf("esperaba allow, obtuvo %s", resp.Decision)
	}
	if resp.Status != "allowed" {
		t.Fatalf("esperaba status allowed, obtuvo %s", resp.Status)
	}
}

func TestSubmitRequestRequireApproval(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// alert.silence es high risk → require_approval por default
	rec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"ops-bot","action_type":"alert.silence","target_resource":"CPU"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var resp requestdto.SubmitResponse
	decJSON(t, rec, &resp)
	if resp.Decision != "require_approval" {
		t.Fatalf("esperaba require_approval, obtuvo %s", resp.Decision)
	}
	if resp.Approval == nil {
		t.Fatal("approval no deberia ser nil")
	}
	if resp.Status != "pending_approval" {
		t.Fatalf("esperaba status pending_approval, obtuvo %s", resp.Status)
	}
	// AI degraded deberia ser true porque usamos stub
	if !resp.AIDegraded {
		t.Fatal("esperaba ai_degraded=true con stub contextualizer")
	}
	if resp.AISummary == "" {
		t.Fatal("esperaba ai_summary no vacio con stub contextualizer")
	}
}

func TestSubmitRequestDeny(t *testing.T) {
	t.Parallel()

	// Política que deniega todas las acciones "deploy.execute"
	actionType := "deploy.execute"
	policies := []requests.PolicyForEval{
		{
			ID:         uuid.New(),
			Name:       "deny-deploys",
			ActionType: &actionType,
			Expression: "true",
			Effect:     "deny",
		},
	}
	mux := setupRequestMuxWithPolicies(policies)

	rec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"ci-bot","action_type":"deploy.execute"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var resp requestdto.SubmitResponse
	decJSON(t, rec, &resp)
	if resp.Decision != "deny" {
		t.Fatalf("esperaba deny, obtuvo %s", resp.Decision)
	}
	if resp.Status != "denied" {
		t.Fatalf("esperaba status denied, obtuvo %s", resp.Status)
	}
}

func TestSubmitRequestWithPolicyAllow(t *testing.T) {
	t.Parallel()

	// Política que permite explicitamente "alert.ack"
	actionType := "alert.ack"
	policies := []requests.PolicyForEval{
		{
			ID:         uuid.New(),
			Name:       "allow-ack",
			ActionType: &actionType,
			Expression: "true",
			Effect:     "allow",
		},
	}
	mux := setupRequestMuxWithPolicies(policies)

	rec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"human","requester_id":"user@co","action_type":"alert.ack"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var resp requestdto.SubmitResponse
	decJSON(t, rec, &resp)
	if resp.Decision != "allow" {
		t.Fatalf("esperaba allow, obtuvo %s", resp.Decision)
	}
	if resp.DecisionReason == "" {
		t.Fatal("esperaba decision_reason no vacio para policy match")
	}
}

// --- Tests de validacion ---

func TestSubmitValidation(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"cuerpo vacio", `{}`, http.StatusBadRequest},
		{"falta action_type", `{"requester_type":"agent","requester_id":"bot"}`, http.StatusBadRequest},
		{"falta requester_type", `{"requester_id":"bot","action_type":"x"}`, http.StatusBadRequest},
		{"falta requester_id", `{"requester_type":"agent","action_type":"x"}`, http.StatusBadRequest},
		{"json invalido", `{bad`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := doReq(t, mux, http.MethodPost, "/v1/requests", tt.body)
			if rec.Code != tt.status {
				t.Fatalf("esperaba %d, obtuvo %d: %s", tt.status, rec.Code, rec.Body.String())
			}

			// Verificar que el error tiene estructura correcta
			var errResp struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			decJSON(t, rec, &errResp)
			if errResp.Code == "" {
				t.Fatal("esperaba code de error no vacio")
			}
			if errResp.Message == "" {
				t.Fatal("esperaba message de error no vacio")
			}
		})
	}
}

func TestSubmitIdempotencyKeyTooLong(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	body := `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/requests", strings.NewReader(body))
	// Clave de idempotencia de 257 caracteres (max es 256)
	req.Header.Set("Idempotency-Key", strings.Repeat("a", 257))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuvo %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Tests de idempotencia ---

func TestIdempotency(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	body := `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`

	// Primera request con clave de idempotencia
	req1 := httptest.NewRequest(http.MethodPost, "/v1/requests", strings.NewReader(body))
	req1.Header.Set("Idempotency-Key", "test-key-123")
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)

	// Segunda request con la misma clave
	req2 := httptest.NewRequest(http.MethodPost, "/v1/requests", strings.NewReader(body))
	req2.Header.Set("Idempotency-Key", "test-key-123")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	var resp1, resp2 requestdto.SubmitResponse
	decJSON(t, rec1, &resp1)
	decJSON(t, rec2, &resp2)

	if resp1.RequestID != resp2.RequestID {
		t.Fatalf("idempotencia fallo: %s vs %s", resp1.RequestID, resp2.RequestID)
	}
	if resp1.Decision != resp2.Decision {
		t.Fatalf("decisiones distintas: %s vs %s", resp1.Decision, resp2.Decision)
	}
}

func TestIdempotencyDifferentKeys(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	body := `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`

	// Dos requests con claves distintas deben generar IDs distintos
	req1 := httptest.NewRequest(http.MethodPost, "/v1/requests", strings.NewReader(body))
	req1.Header.Set("Idempotency-Key", "key-alpha")
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/v1/requests", strings.NewReader(body))
	req2.Header.Set("Idempotency-Key", "key-beta")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	var resp1, resp2 requestdto.SubmitResponse
	decJSON(t, rec1, &resp1)
	decJSON(t, rec2, &resp2)

	if resp1.RequestID == resp2.RequestID {
		t.Fatalf("claves distintas deben generar IDs distintos, ambos son %s", resp1.RequestID)
	}
}

// --- Tests de GetByID ---

func TestGetByIDHappyPath(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Crear una request primero
	createRec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuvo %d", createRec.Code)
	}
	var submitResp requestdto.SubmitResponse
	decJSON(t, createRec, &submitResp)

	// Obtener la request por ID
	rec := doReq(t, mux, http.MethodGet, "/v1/requests/"+submitResp.RequestID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var resp requestdto.RequestResponse
	decJSON(t, rec, &resp)
	if resp.ID != submitResp.RequestID {
		t.Fatalf("esperaba id %s, obtuvo %s", submitResp.RequestID, resp.ID)
	}
	if resp.RequesterType != "agent" {
		t.Fatalf("esperaba requester_type agent, obtuvo %s", resp.RequesterType)
	}
	if resp.RequesterID != "bot" {
		t.Fatalf("esperaba requester_id bot, obtuvo %s", resp.RequesterID)
	}
	if resp.ActionType != "alert.escalate" {
		t.Fatalf("esperaba action_type alert.escalate, obtuvo %s", resp.ActionType)
	}
	if resp.Decision != "allow" {
		t.Fatalf("esperaba decision allow, obtuvo %s", resp.Decision)
	}
	if resp.CreatedAt == "" {
		t.Fatal("esperaba created_at no vacio")
	}
	if resp.UpdatedAt == "" {
		t.Fatal("esperaba updated_at no vacio")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	rec := doReq(t, mux, http.MethodGet, "/v1/requests/00000000-0000-0000-0000-000000000000", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuvo %d", rec.Code)
	}

	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decJSON(t, rec, &errResp)
	if errResp.Code != "NOT_FOUND" {
		t.Fatalf("esperaba code NOT_FOUND, obtuvo %s", errResp.Code)
	}
}

func TestGetByIDInvalidUUID(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	rec := doReq(t, mux, http.MethodGet, "/v1/requests/not-a-uuid", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decJSON(t, rec, &errResp)
	if errResp.Code != "VALIDATION" {
		t.Fatalf("esperaba code VALIDATION, obtuvo %s", errResp.Code)
	}
}

// --- Tests de List ---

func TestListRequestsEmpty(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	rec := doReq(t, mux, http.MethodGet, "/v1/requests", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}

	var listResp struct {
		Data []requestdto.RequestResponse `json:"data"`
	}
	decJSON(t, rec, &listResp)
	if len(listResp.Data) != 0 {
		t.Fatalf("esperaba 0, obtuvo %d", len(listResp.Data))
	}
}

func TestListRequests(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Crear dos requests
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"human","requester_id":"user@co","action_type":"incident.resolve"}`)

	rec := doReq(t, mux, http.MethodGet, "/v1/requests", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}

	var listResp struct {
		Data []requestdto.RequestResponse `json:"data"`
	}
	decJSON(t, rec, &listResp)
	if len(listResp.Data) != 2 {
		t.Fatalf("esperaba 2, obtuvo %d", len(listResp.Data))
	}
}

func TestListRequestsFilterByStatus(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// alert.escalate → low risk → allowed
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	// alert.silence → high risk → pending_approval
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.silence"}`)

	// Filtrar por status=allowed
	rec := doReq(t, mux, http.MethodGet, "/v1/requests?status=allowed", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}

	var listResp struct {
		Data []requestdto.RequestResponse `json:"data"`
	}
	decJSON(t, rec, &listResp)
	if len(listResp.Data) != 1 {
		t.Fatalf("esperaba 1 request allowed, obtuvo %d", len(listResp.Data))
	}
	if listResp.Data[0].Status != "allowed" {
		t.Fatalf("esperaba status allowed, obtuvo %s", listResp.Data[0].Status)
	}
}

func TestListRequestsFilterByActionType(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"human","requester_id":"user@co","action_type":"incident.resolve"}`)

	// Filtrar por action_type
	rec := doReq(t, mux, http.MethodGet, "/v1/requests?action_type=alert.escalate", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}

	var listResp struct {
		Data []requestdto.RequestResponse `json:"data"`
	}
	decJSON(t, rec, &listResp)
	if len(listResp.Data) != 1 {
		t.Fatalf("esperaba 1, obtuvo %d", len(listResp.Data))
	}
	if listResp.Data[0].ActionType != "alert.escalate" {
		t.Fatalf("esperaba action_type alert.escalate, obtuvo %s", listResp.Data[0].ActionType)
	}
}

func TestListRequestsWithLimit(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Crear 3 requests
	for i := 0; i < 3; i++ {
		doReq(t, mux, http.MethodPost, "/v1/requests",
			fmt.Sprintf(`{"requester_type":"agent","requester_id":"bot-%d","action_type":"alert.escalate"}`, i))
	}

	// Pedir con limit=2
	rec := doReq(t, mux, http.MethodGet, "/v1/requests?limit=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}

	var listResp struct {
		Data []requestdto.RequestResponse `json:"data"`
	}
	decJSON(t, rec, &listResp)
	if len(listResp.Data) != 2 {
		t.Fatalf("esperaba 2, obtuvo %d", len(listResp.Data))
	}
}

func TestListRequestsInvalidLimit(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Limit invalido se ignora y se usa el default
	rec := doReq(t, mux, http.MethodGet, "/v1/requests?limit=abc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}
}

func TestListRequestsNegativeLimit(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Limit negativo se ignora y se usa el default
	rec := doReq(t, mux, http.MethodGet, "/v1/requests?limit=-5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}
}

func TestListRequestsLimitExceedsMax(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Limit que excede MaxListLimit (1000) se ignora y se usa el default
	rec := doReq(t, mux, http.MethodGet, "/v1/requests?limit=5000", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}
}

func TestListRequestsCombinedFilters(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Crear varias requests con distintos tipos
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"human","requester_id":"user@co","action_type":"alert.escalate"}`)
	doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"incident.resolve"}`)

	// Filtrar por status=allowed y action_type=alert.escalate
	rec := doReq(t, mux, http.MethodGet, "/v1/requests?status=allowed&action_type=alert.escalate", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d", rec.Code)
	}

	var listResp struct {
		Data []requestdto.RequestResponse `json:"data"`
	}
	decJSON(t, rec, &listResp)
	// Las 2 alert.escalate deberian tener status=allowed (low risk)
	if len(listResp.Data) != 2 {
		t.Fatalf("esperaba 2, obtuvo %d", len(listResp.Data))
	}
	for _, d := range listResp.Data {
		if d.Status != "allowed" {
			t.Fatalf("esperaba status allowed, obtuvo %s", d.Status)
		}
		if d.ActionType != "alert.escalate" {
			t.Fatalf("esperaba action_type alert.escalate, obtuvo %s", d.ActionType)
		}
	}
}

// --- Tests de ReportResult ---

func TestReportResultSuccess(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Crear request
	createRec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	var submitResp requestdto.SubmitResponse
	decJSON(t, createRec, &submitResp)

	// Reportar resultado exitoso
	resultBody := `{"success":true,"result":{"output":"escalated"},"duration_ms":150}`
	rec := doReq(t, mux, http.MethodPost, "/v1/requests/"+submitResp.RequestID+"/result", resultBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var resultResp map[string]string
	decJSON(t, rec, &resultResp)
	if resultResp["status"] != "ok" {
		t.Fatalf("esperaba status ok, obtuvo %s", resultResp["status"])
	}

	// Verificar que el estado cambio a executed
	getRec := doReq(t, mux, http.MethodGet, "/v1/requests/"+submitResp.RequestID, "")
	var getResp requestdto.RequestResponse
	decJSON(t, getRec, &getResp)
	if getResp.Status != "executed" {
		t.Fatalf("esperaba status executed, obtuvo %s", getResp.Status)
	}
}

func TestReportResultFailure(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Crear request
	createRec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	var submitResp requestdto.SubmitResponse
	decJSON(t, createRec, &submitResp)

	// Reportar resultado fallido
	resultBody := `{"success":false,"error_message":"timeout connecting to target"}`
	rec := doReq(t, mux, http.MethodPost, "/v1/requests/"+submitResp.RequestID+"/result", resultBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	// Verificar que el estado cambio a failed
	getRec := doReq(t, mux, http.MethodGet, "/v1/requests/"+submitResp.RequestID, "")
	var getResp requestdto.RequestResponse
	decJSON(t, getRec, &getResp)
	if getResp.Status != "failed" {
		t.Fatalf("esperaba status failed, obtuvo %s", getResp.Status)
	}
}

func TestReportResultNotFound(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	resultBody := `{"success":true,"result":{"ok":true}}`
	rec := doReq(t, mux, http.MethodPost, "/v1/requests/00000000-0000-0000-0000-000000000000/result", resultBody)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuvo %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReportResultInvalidUUID(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	resultBody := `{"success":true}`
	rec := doReq(t, mux, http.MethodPost, "/v1/requests/not-a-uuid/result", resultBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Code string `json:"code"`
	}
	decJSON(t, rec, &errResp)
	if errResp.Code != "VALIDATION" {
		t.Fatalf("esperaba code VALIDATION, obtuvo %s", errResp.Code)
	}
}

func TestReportResultInvalidJSON(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	// Crear una request primero
	createRec := doReq(t, mux, http.MethodPost, "/v1/requests", `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`)
	var submitResp requestdto.SubmitResponse
	decJSON(t, createRec, &submitResp)

	// Enviar JSON invalido
	rec := doReq(t, mux, http.MethodPost, "/v1/requests/"+submitResp.RequestID+"/result", `{bad json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuvo %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Tests de Submit con params ---

func TestSubmitRequestWithParams(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	body := `{
		"requester_type":"service",
		"requester_id":"monitoring-svc",
		"requester_name":"Monitoring Service",
		"action_type":"alert.escalate",
		"target_system":"pagerduty",
		"target_resource":"team-oncall",
		"params":{"severity":"critical","alert_id":"12345"},
		"reason":"High CPU usage detected",
		"context":"Production server us-east-1"
	}`
	rec := doReq(t, mux, http.MethodPost, "/v1/requests", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuvo %d: %s", rec.Code, rec.Body.String())
	}

	var resp requestdto.SubmitResponse
	decJSON(t, rec, &resp)

	// Verificar que los datos se persisten correctamente via GetByID
	getRec := doReq(t, mux, http.MethodGet, "/v1/requests/"+resp.RequestID, "")
	var getResp requestdto.RequestResponse
	decJSON(t, getRec, &getResp)

	if getResp.RequesterType != "service" {
		t.Fatalf("esperaba requester_type service, obtuvo %s", getResp.RequesterType)
	}
	if getResp.RequesterName != "Monitoring Service" {
		t.Fatalf("esperaba requester_name Monitoring Service, obtuvo %s", getResp.RequesterName)
	}
	if getResp.TargetSystem != "pagerduty" {
		t.Fatalf("esperaba target_system pagerduty, obtuvo %s", getResp.TargetSystem)
	}
	if getResp.TargetResource != "team-oncall" {
		t.Fatalf("esperaba target_resource team-oncall, obtuvo %s", getResp.TargetResource)
	}
	if getResp.Reason != "High CPU usage detected" {
		t.Fatalf("esperaba reason correcto, obtuvo %s", getResp.Reason)
	}
}

// --- Tests de Submit sin idempotency (multiples requests generan IDs distintos) ---

func TestSubmitWithoutIdempotencyKey(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	body := `{"requester_type":"agent","requester_id":"bot","action_type":"alert.escalate"}`

	rec1 := doReq(t, mux, http.MethodPost, "/v1/requests", body)
	rec2 := doReq(t, mux, http.MethodPost, "/v1/requests", body)

	var resp1, resp2 requestdto.SubmitResponse
	decJSON(t, rec1, &resp1)
	decJSON(t, rec2, &resp2)

	if resp1.RequestID == resp2.RequestID {
		t.Fatalf("sin idempotency key, cada submit debe generar un ID distinto, ambos son %s", resp1.RequestID)
	}
}

// --- Tests de risk level ---

func TestSubmitRiskLevels(t *testing.T) {
	t.Parallel()
	mux := setupRequestMux()

	tests := []struct {
		name       string
		actionType string
		wantRisk   string
		wantDecision string
	}{
		{
			name:         "cascade risk: accion desconocida (allow con cualquier nivel)",
			actionType:   "custom.action",
			wantRisk:     "", // cascade: nivel varía según hora y contexto
			wantDecision: "allow",
		},
		{
			name:         "high risk: alert.silence",
			actionType:   "alert.silence",
			wantRisk:     "high",
			wantDecision: "require_approval",
		},
		{
			name:         "high risk: runbook.execute",
			actionType:   "runbook.execute",
			wantRisk:     "high",
			wantDecision: "require_approval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := fmt.Sprintf(`{"requester_type":"agent","requester_id":"bot","action_type":"%s"}`, tt.actionType)
			rec := doReq(t, mux, http.MethodPost, "/v1/requests", body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("esperaba 201, obtuvo %d: %s", rec.Code, rec.Body.String())
			}

			var resp requestdto.SubmitResponse
			decJSON(t, rec, &resp)
			if tt.wantRisk != "" && resp.RiskLevel != tt.wantRisk {
				t.Fatalf("esperaba risk %s, obtuvo %s", tt.wantRisk, resp.RiskLevel)
			}
			if resp.Decision != tt.wantDecision {
				t.Fatalf("esperaba decision %s, obtuvo %s", tt.wantDecision, resp.Decision)
			}
		})
	}
}
