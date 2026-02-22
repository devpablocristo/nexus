package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cfgpkg "world-sim/internal/config"
)

func TestInternalAuthMiddleware(t *testing.T) {
	s := &Server{cfg: cfgpkg.Config{InternalKey: "secret"}}

	h := s.withAuth(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	reqNoKey := httptest.NewRequest(http.MethodGet, "/secure", nil)
	wNoKey := httptest.NewRecorder()
	h.ServeHTTP(wNoKey, reqNoKey)
	if wNoKey.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", wNoKey.Code)
	}

	reqOK := httptest.NewRequest(http.MethodGet, "/secure", nil)
	reqOK.Header.Set("X-WorldSim-Internal-Key", "secret")
	wOK := httptest.NewRecorder()
	h.ServeHTTP(wOK, reqOK)
	if wOK.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", wOK.Code)
	}
}
