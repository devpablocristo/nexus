package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	policydomain "nexus-core/internal/policy/usecases/domain"
	"nexus/pkg/types"
)

type policySvcStub struct{ called bool }

func (s *policySvcStub) CreateForTool(context.Context, uuid.UUID, string, CreateRequest) (policydomain.Policy, error) {
	s.called = true
	return policydomain.Policy{}, nil
}
func (s *policySvcStub) ListForTool(context.Context, uuid.UUID, string) ([]policydomain.Policy, error) {
	s.called = true
	return []policydomain.Policy{}, nil
}
func (s *policySvcStub) UpdateByID(context.Context, uuid.UUID, uuid.UUID, PolicyPatch) (policydomain.Policy, error) {
	s.called = true
	return policydomain.Policy{}, nil
}

func TestPolicyListAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &policySvcStub{}
	h := NewHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"tools:read"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/tools/x/policies", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"policy:read"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/tools/x/policies", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
