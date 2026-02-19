package secrets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	secretdomain "nexus-gateway/internal/secrets/usecases/domain"
	"nexus-gateway/pkg/types"
)

type secretsSvcStub struct{ called bool }

func (s *secretsSvcStub) UpsertForTool(context.Context, uuid.UUID, string, string, string, string, bool) (secretdomain.ToolSecret, error) {
	s.called = true
	return secretdomain.ToolSecret{}, nil
}

func (s *secretsSvcStub) ListForTool(context.Context, uuid.UUID, string) ([]secretdomain.ToolSecret, error) {
	s.called = true
	return []secretdomain.ToolSecret{}, nil
}

func (s *secretsSvcStub) DeleteForTool(context.Context, uuid.UUID, string, string) error {
	s.called = true
	return nil
}

func TestSecretsListAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &secretsSvcStub{}
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
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/tools/echo/secrets", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}
	if svc.called {
		t.Fatalf("service should not be called when forbidden")
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:secrets"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/tools/echo/secrets", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}

func TestSecretsUpsertAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &secretsSvcStub{}
	h := NewHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:secrets"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)

	body := `{"secret_type":"header","key_name":"X-Token","value":"abc"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/tools/echo/secrets", strings.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
