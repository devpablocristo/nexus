package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	tooldomain "data-plane/internal/tool/usecases/domain"
	"nexus/pkg/types"
)

type toolSvcStub struct{ called bool }

func (s *toolSvcStub) Create(context.Context, uuid.UUID, CreateRequest) (tooldomain.Tool, error) {
	s.called = true
	return tooldomain.Tool{}, nil
}
func (s *toolSvcStub) List(context.Context, uuid.UUID) ([]tooldomain.Tool, error) {
	s.called = true
	return []tooldomain.Tool{}, nil
}
func (s *toolSvcStub) GetByName(context.Context, uuid.UUID, string) (tooldomain.Tool, error) {
	s.called = true
	return tooldomain.Tool{}, nil
}
func (s *toolSvcStub) GetByID(_ context.Context, _, _ uuid.UUID) (tooldomain.Tool, error) {
	s.called = true
	return tooldomain.Tool{}, nil
}
func (s *toolSvcStub) ResolveRef(_ context.Context, _ uuid.UUID, _ string) (tooldomain.Tool, error) {
	s.called = true
	return tooldomain.Tool{}, nil
}
func (s *toolSvcStub) UpdateByName(context.Context, uuid.UUID, string, ToolPatch) (tooldomain.Tool, error) {
	s.called = true
	return tooldomain.Tool{}, nil
}
func (s *toolSvcStub) DeleteByName(context.Context, uuid.UUID, string) error {
	s.called = true
	return nil
}

func TestToolListAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &toolSvcStub{}
	h := NewHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"other:read"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)

	req := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
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
		c.Set(string(types.CtxKeyScopes), []string{"tools:read"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/tools", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
