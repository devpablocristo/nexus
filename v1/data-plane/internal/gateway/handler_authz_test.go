package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdomain "data-plane/internal/gateway/usecases/domain"
	"nexus/pkg/types"
)

type gatewaySvcStub struct{ called bool }

func (s *gatewaySvcStub) Run(context.Context, uuid.UUID, gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	s.called = true
	return gwdomain.RunResponse{RequestID: "r1", ToolName: "echo", Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusSuccess, HTTPStatus: http.StatusOK}, nil
}
func (s *gatewaySvcStub) Simulate(context.Context, uuid.UUID, gwdomain.RunRequest) (gwdomain.SimulateResponse, error) {
	s.called = true
	return gwdomain.SimulateResponse{RequestID: "r1", ToolName: "echo", Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusSuccess, HTTPStatus: http.StatusOK}, nil
}
func (s *gatewaySvcStub) ExecuteIntent(context.Context, uuid.UUID, uuid.UUID, int) (gwdomain.RunResponse, error) {
	s.called = true
	return gwdomain.RunResponse{RequestID: "r1", ToolName: "echo", Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusSuccess, HTTPStatus: http.StatusOK}, nil
}
func (s *gatewaySvcStub) ExecuteIntentWithLease(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int) (gwdomain.RunResponse, error) {
	s.called = true
	return gwdomain.RunResponse{RequestID: "r1", ToolName: "echo", Decision: gwdomain.DecisionAllow, Status: gwdomain.RunStatusSuccess, HTTPStatus: http.StatusOK}, nil
}
func (s *gatewaySvcStub) IssueExecutionLease(context.Context, uuid.UUID, uuid.UUID) (gwdomain.ExecutionLease, error) {
	s.called = true
	return gwdomain.ExecutionLease{ID: uuid.New(), IntentID: uuid.New(), Status: gwdomain.ExecutionLeaseStatusActive}, nil
}
func (s *gatewaySvcStub) GetIntent(context.Context, uuid.UUID, uuid.UUID) (gwdomain.ExecutionIntent, error) {
	s.called = true
	return gwdomain.ExecutionIntent{ID: uuid.New()}, nil
}
func (s *gatewaySvcStub) ListIntents(context.Context, uuid.UUID, int) ([]gwdomain.ExecutionIntent, error) {
	s.called = true
	return []gwdomain.ExecutionIntent{}, nil
}
func (s *gatewaySvcStub) GetIntentPreflight(context.Context, uuid.UUID, uuid.UUID) (gwdomain.PreflightReview, error) {
	s.called = true
	return gwdomain.PreflightReview{IntentID: uuid.New(), Status: gwdomain.PreflightStatusPassed}, nil
}

func TestGatewayRunAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &gatewaySvcStub{}
	h := NewHandler(svc)
	body := `{"tool_name":"echo","input":{},"context":{}}`

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"tools:read"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/run", strings.NewReader(body)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"gateway:run"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/run", strings.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}

func TestGatewayIntentsReadAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &gatewaySvcStub{}
	h := NewHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"gateway:run"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/run/intents", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:console:read"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/run/intents", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}

func TestGatewayIntentPreflightReadAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &gatewaySvcStub{}
	h := NewHandler(svc)
	intentID := uuid.NewString()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"gateway:run"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/run/intents/"+intentID+"/preflight", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:console:read"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/run/intents/"+intentID+"/preflight", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}

func TestGatewayIntentLeaseAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &gatewaySvcStub{}
	h := NewHandler(svc)
	intentID := uuid.NewString()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:console:read"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/run/intents/"+intentID+"/lease", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"gateway:run"})
		c.Next()
	})
	v1 = r.Group("/v1")
	h.Register(v1)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/run/intents/"+intentID+"/lease", nil))
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
