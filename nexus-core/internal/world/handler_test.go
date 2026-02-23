package world

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ginmw "nexus-core/pkg/http/middlewares/gin"
	"nexus-core/pkg/types"
)

type fakeWorldService struct{}

func (f fakeWorldService) ListRuns(_ context.Context, _ uuid.UUID, _ string, _ int, _ string) (any, error) {
	return map[string]any{"items": []any{}, "next_cursor": ""}, nil
}
func (f fakeWorldService) GetState(_ context.Context, _ uuid.UUID, _ string, _ string, _ *int64) (any, error) {
	return map[string]any{"run_id": "r1", "step_id": 0, "state_hash": "h"}, nil
}
func (f fakeWorldService) GetEvents(_ context.Context, _ uuid.UUID, _ string, _ string, _ int64, _ int) (any, error) {
	return map[string]any{"items": []any{}, "next_seq": 0}, nil
}
func (f fakeWorldService) CreateRun(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) (any, error) {
	return map[string]any{"run_id": "r1"}, nil
}
func (f fakeWorldService) Replay(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) (any, error) {
	return map[string]any{"run_id": "r1", "replayed_moves": 0}, nil
}

func TestHandler_ReadEndpointRequiresReadScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fakeWorldService{})
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
	h := NewHandler(fakeWorldService{})
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
	h := NewHandler(fakeWorldService{})
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
	h := NewHandler(fakeWorldService{})
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
