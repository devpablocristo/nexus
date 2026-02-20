package a2a

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/pkg/types"
)

type a2aSvcStub struct{ called bool }

func (s *a2aSvcStub) CallTool(context.Context, uuid.UUID, gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	s.called = true
	return gwdomain.RunResponse{
		RequestID:  "req-1",
		Decision:   gwdomain.DecisionAllow,
		ToolName:   "echo",
		Status:     gwdomain.RunStatusSuccess,
		HTTPStatus: http.StatusOK,
	}, nil
}

func TestA2ACallAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &a2aSvcStub{}
	h := NewHandler(svc)

	body := `{"tool_name":"echo","input":{"hello":"world"}}`

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"gateway:run"})
		c.Next()
	})
	h.Register(r.Group(""))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/a2a/call", strings.NewReader(body)))
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
		c.Set(string(types.CtxKeyScopes), []string{"a2a:call"})
		c.Next()
	})
	h.Register(r.Group(""))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/a2a/call", strings.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
