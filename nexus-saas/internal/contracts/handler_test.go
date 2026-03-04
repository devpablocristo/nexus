package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-saas/cmd/config"
	admindomain "nexus-saas/internal/admin/usecases/domain"
)

type entitlementsStub struct {
	settings admindomain.TenantSettings
	ok       bool
	err      error
}

func (s *entitlementsStub) GetTenantSettings(_ context.Context, _ uuid.UUID) (admindomain.TenantSettings, bool, error) {
	return s.settings, s.ok, s.err
}

type usageStub struct {
	seen map[string]int
}

func (s *usageStub) IncrementEvent(_ context.Context, eventID string, _ uuid.UUID, _ string) error {
	if s.seen == nil {
		s.seen = map[string]int{}
	}
	s.seen[eventID]++
	return nil
}

func newRouterForTest(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterInternal(r)
	return r
}

func TestInternalEndpoints_RequireKey(t *testing.T) {
	h := &Handler{
		cfg:      config.ServiceConfig{SaaSInternalKey: "k1"},
		admin:    &entitlementsStub{},
		metering: &usageStub{},
	}
	r := newRouterForTest(h)
	req := httptest.NewRequest(http.MethodGet, "/internal/entitlements/"+uuid.NewString(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestIngestUsage_IsIdempotentByEventID(t *testing.T) {
	usage := &usageStub{}
	h := &Handler{
		cfg:      config.ServiceConfig{SaaSInternalKey: "k1"},
		admin:    &entitlementsStub{},
		metering: usage,
	}
	r := newRouterForTest(h)
	body := map[string]any{
		"event_id": "ev-1",
		"org_id":   uuid.NewString(),
		"counter":  "api_calls",
	}
	raw, _ := json.Marshal(body)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/internal/usage/events", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-NEXUS-SAAS-KEY", "k1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", w.Code)
		}
	}
	if usage.seen["ev-1"] != 2 {
		t.Fatalf("expected handler to pass event twice to sink, got %d", usage.seen["ev-1"])
	}
}

