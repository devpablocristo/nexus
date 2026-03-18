package dashboard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/devpablocristo/nexus/v3/review/internal/dashboard"
	requestdomain "github.com/devpablocristo/nexus/v3/review/internal/requests/usecases/domain"
)

type fakeRequestLister struct {
	mu       sync.RWMutex
	requests []requestdomain.Request
}

func (r *fakeRequestLister) List(_ context.Context, _, _ string, _ int) ([]requestdomain.Request, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.requests, nil
}

func TestDashboardEmpty(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	dashboard.NewHandler(&fakeRequestLister{}).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/metrics/summary", nil))
	if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d", rec.Code) }
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["total_requests"].(float64) != 0 { t.Fatalf("expected 0, got %v", resp["total_requests"]) }
}

func TestDashboardWithData(t *testing.T) {
	t.Parallel()
	lister := &fakeRequestLister{requests: []requestdomain.Request{
		{ID: uuid.New(), Status: requestdomain.StatusAllowed},
		{ID: uuid.New(), Status: requestdomain.StatusAllowed},
		{ID: uuid.New(), Status: requestdomain.StatusDenied},
		{ID: uuid.New(), Status: requestdomain.StatusPendingApproval},
		{ID: uuid.New(), Status: requestdomain.StatusExecuted},
	}}
	mux := http.NewServeMux()
	dashboard.NewHandler(lister).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/metrics/summary", nil))
	if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d", rec.Code) }
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["total_requests"].(float64) != 5 { t.Fatalf("expected 5, got %v", resp["total_requests"]) }
	if resp["allowed"].(float64) != 2 { t.Fatalf("expected 2 allowed, got %v", resp["allowed"]) }
}
