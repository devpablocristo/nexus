package world

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ginmw "nexus-core/pkg/http/middlewares/gin"
	"nexus-core/pkg/types"
)

func fakeUsecases(t *testing.T) *Usecases {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/admin/run/runs":
			_, _ = w.Write([]byte(`{"items":[],"next_cursor":""}`))
		case r.URL.Path == "/admin/run/state":
			_, _ = w.Write([]byte(`{"run_id":"r1","step_id":0,"state_hash":"h"}`))
		case r.URL.Path == "/admin/run/events":
			_, _ = w.Write([]byte(`{"items":[],"next_seq":0}`))
		case r.URL.Path == "/admin/run/create":
			_, _ = w.Write([]byte(`{"run_id":"r1"}`))
		case r.URL.Path == "/admin/run/replay":
			_, _ = w.Write([]byte(`{"run_id":"r1","replayed_moves":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return NewUsecases(Config{BaseURL: srv.URL, Timeout: 2 * time.Second})
}

func TestHandler_ReadEndpointRequiresReadScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fakeUsecases(t))
	r := gin.New()
	r.Use(ginmw.RequestID())
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"tools:read"})
		c.Next()
	})
	h.Register(v1)

	req := httptest.NewRequest(http.MethodGet, "/v1/world/runs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing %s scope, got %d", "admin:console:read", w.Code)
	}
}

func TestHandler_WriteEndpointRequiresWriteScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fakeUsecases(t))
	r := gin.New()
	r.Use(ginmw.RequestID())
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:console:read"})
		c.Next()
	})
	h.Register(v1)

	req := httptest.NewRequest(http.MethodPost, "/v1/world/run/create", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing %s scope, got %d", "admin:console:write", w.Code)
	}
}

func TestHandler_ListRunsWithReadScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fakeUsecases(t))
	r := gin.New()
	r.Use(ginmw.RequestID())
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:console:read"})
		c.Next()
	})
	h.Register(v1)

	req := httptest.NewRequest(http.MethodGet, "/v1/world/runs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

func TestHandler_StreamEventsRequiresRunID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fakeUsecases(t))
	r := gin.New()
	r.Use(ginmw.RequestID())
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"admin:console:read"})
		c.Next()
	})
	h.Register(v1)

	req := httptest.NewRequest(http.MethodGet, "/v1/world/events/stream", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing run_id, got %d", w.Code)
	}
}
