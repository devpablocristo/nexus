package policies_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/devpablocristo/nexus/v3/review/internal/policies"
	policydomain "github.com/devpablocristo/nexus/v3/review/internal/policies/usecases/domain"
	policydto "github.com/devpablocristo/nexus/v3/review/internal/policies/handler/dto"
)

// fakeRepo implementa policies.Repository para tests sin postgres.
type fakeRepo struct {
	mu    sync.RWMutex
	byID  map[uuid.UUID]policydomain.Policy
	order []uuid.UUID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: make(map[uuid.UUID]policydomain.Policy)}
}

func (r *fakeRepo) Create(_ context.Context, p policydomain.Policy) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if p.ID == uuid.Nil { p.ID = uuid.New() }
	if p.CreatedAt.IsZero() { p.CreatedAt = now }
	p.UpdatedAt = now
	r.byID[p.ID] = p
	r.order = append(r.order, p.ID)
	return p, nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok { return policydomain.Policy{}, policies.ErrNotFound }
	return p, nil
}

func (r *fakeRepo) List(_ context.Context, filters policies.ListFilters) ([]policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []policydomain.Policy
	for _, id := range r.order {
		p := r.byID[id]
		if !filters.IncludeArchived && p.ArchivedAt != nil { continue }
		if filters.EnabledOnly && !p.Enabled { continue }
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Priority < out[j].Priority })
	return out, nil
}

func (r *fakeRepo) Update(_ context.Context, p policydomain.Policy) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[p.ID]; !ok { return policydomain.Policy{}, policies.ErrNotFound }
	p.UpdatedAt = time.Now().UTC()
	r.byID[p.ID] = p
	return p, nil
}

func (r *fakeRepo) DeleteByID(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok { return policies.ErrNotFound }
	delete(r.byID, id)
	return nil
}

func (r *fakeRepo) ArchiveByID(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok { return policies.ErrNotFound }
	if p.ArchivedAt != nil { return nil }
	now := time.Now().UTC()
	p.ArchivedAt = &now
	r.byID[id] = p
	return nil
}

func (r *fakeRepo) RestoreByID(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok { return policies.ErrNotFound }
	if p.ArchivedAt == nil { return nil }
	p.ArchivedAt = nil
	r.byID[id] = p
	return nil
}

func setupPolicyMux() *http.ServeMux {
	uc := policies.NewUsecases(newFakeRepo())
	mux := http.NewServeMux()
	policies.NewHandler(uc).Register(mux)
	return mux
}

func TestPolicyCRUDLifecycle(t *testing.T) {
	t.Parallel()
	mux := setupPolicyMux()

	// Create — 201
	rec := doRequest(t, mux, http.MethodPost, "/v1/policies", `{"name":"test-policy","expression":"true","effect":"allow","priority":10,"enabled":true}`)
	if rec.Code != http.StatusCreated { t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String()) }
	var created policydto.PolicyResponse
	decodeJSON(t, rec, &created)
	policyID := created.ID

	// Get — 200
	rec = doRequest(t, mux, http.MethodGet, "/v1/policies/"+policyID, "")
	if rec.Code != http.StatusOK { t.Fatalf("get: expected 200, got %d", rec.Code) }

	// List — 200
	rec = doRequest(t, mux, http.MethodGet, "/v1/policies", "")
	if rec.Code != http.StatusOK { t.Fatalf("list: expected 200, got %d", rec.Code) }

	// Update — 200
	rec = doRequest(t, mux, http.MethodPatch, "/v1/policies/"+policyID, `{"name":"updated"}`)
	if rec.Code != http.StatusOK { t.Fatalf("update: expected 200, got %d: %s", rec.Code, rec.Body.String()) }

	// Archive — 204
	rec = doRequest(t, mux, http.MethodPost, "/v1/policies/"+policyID+"/archive", "")
	if rec.Code != http.StatusNoContent { t.Fatalf("archive: expected 204, got %d", rec.Code) }

	// Archive idempotente
	rec = doRequest(t, mux, http.MethodPost, "/v1/policies/"+policyID+"/archive", "")
	if rec.Code != http.StatusNoContent { t.Fatalf("archive idempotent: expected 204, got %d", rec.Code) }

	// List excluye archivados
	rec = doRequest(t, mux, http.MethodGet, "/v1/policies", "")
	var listResp struct{ Data []policydto.PolicyResponse }
	decodeJSON(t, rec, &listResp)
	if len(listResp.Data) != 0 { t.Fatalf("list: expected 0 active, got %d", len(listResp.Data)) }

	// List con archived=true
	rec = doRequest(t, mux, http.MethodGet, "/v1/policies?archived=true", "")
	decodeJSON(t, rec, &listResp)
	if len(listResp.Data) != 1 { t.Fatalf("list archived: expected 1, got %d", len(listResp.Data)) }

	// Restore — 204
	rec = doRequest(t, mux, http.MethodPost, "/v1/policies/"+policyID+"/restore", "")
	if rec.Code != http.StatusNoContent { t.Fatalf("restore: expected 204, got %d", rec.Code) }

	// Delete (hard) — 204
	rec = doRequest(t, mux, http.MethodDelete, "/v1/policies/"+policyID, "")
	if rec.Code != http.StatusNoContent { t.Fatalf("delete: expected 204, got %d", rec.Code) }

	// Get after delete — 404
	rec = doRequest(t, mux, http.MethodGet, "/v1/policies/"+policyID, "")
	if rec.Code != http.StatusNotFound { t.Fatalf("get after delete: expected 404, got %d", rec.Code) }
}

func TestPolicyValidation(t *testing.T) {
	t.Parallel()
	mux := setupPolicyMux()
	tests := []struct{ name, body string; status int }{
		{"empty body", `{}`, 400},
		{"missing expression", `{"name":"x","effect":"allow"}`, 400},
		{"invalid json", `{bad`, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := doRequest(t, mux, http.MethodPost, "/v1/policies", tt.body)
			if rec.Code != tt.status { t.Fatalf("expected %d, got %d", tt.status, rec.Code) }
		})
	}
}

func TestPolicyNotFound(t *testing.T) {
	t.Parallel()
	mux := setupPolicyMux()
	fakeID := "00000000-0000-0000-0000-000000000000"
	tests := []struct{ name, method, path string; status int }{
		{"get", http.MethodGet, "/v1/policies/" + fakeID, 404},
		{"delete", http.MethodDelete, "/v1/policies/" + fakeID, 404},
		{"archive", http.MethodPost, "/v1/policies/" + fakeID + "/archive", 404},
		{"restore", http.MethodPost, "/v1/policies/" + fakeID + "/restore", 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := doRequest(t, mux, tt.method, tt.path, "")
			if rec.Code != tt.status { t.Fatalf("expected %d, got %d", tt.status, rec.Code) }
		})
	}
}

func doRequest(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" { r = strings.NewReader(body) }
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, r))
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil { t.Fatalf("decode: %v, body: %s", err, rec.Body.String()) }
}
