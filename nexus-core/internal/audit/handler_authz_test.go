package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	auditdomain "nexus-core/internal/audit/usecases/domain"
	"nexus/pkg/types"
)

type auditSvcStub struct{ called bool }

func (s *auditSvcStub) Query(context.Context, uuid.UUID, auditdomain.Query) ([]auditdomain.AuditEvent, error) {
	s.called = true
	return []auditdomain.AuditEvent{}, nil
}
func (s *auditSvcStub) StreamByFilters(context.Context, uuid.UUID, auditdomain.Query, int, func(auditdomain.AuditEvent) error) error {
	s.called = true
	return nil
}

func TestAuditQueryAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &auditSvcStub{}
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
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/audit", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"audit:read"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/audit", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
