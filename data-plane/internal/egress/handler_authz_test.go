package egress

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus/pkg/types"
)

type egressSvcStub struct{ called bool }

func (s *egressSvcStub) UpsertRule(context.Context, uuid.UUID, string, string, bool) error {
	s.called = true
	return nil
}
func (s *egressSvcStub) ListRules(context.Context, uuid.UUID, string) ([]string, error) {
	s.called = true
	return []string{"mock-tools"}, nil
}
func (s *egressSvcStub) DeleteRule(context.Context, uuid.UUID, string, string) error {
	s.called = true
	return nil
}
func (s *egressSvcStub) IsHostAllowed(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	s.called = true
	return true, nil
}

func TestEgressListAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &egressSvcStub{}
	h := NewHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"policy:read"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/tools/x/egress-rules", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"egress:read"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/tools/x/egress-rules", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
