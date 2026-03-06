package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus/pkg/types"
)

type a2aContractStub struct {
	lastReq gwdomain.RunRequest
}

func (s *a2aContractStub) CallTool(_ context.Context, _ uuid.UUID, req gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	s.lastReq = req
	return gwdomain.RunResponse{
		RequestID:  "req-a2a",
		Decision:   gwdomain.DecisionAllow,
		ToolName:   req.ToolName,
		Status:     gwdomain.RunStatusSuccess,
		HTTPStatus: http.StatusOK,
		Idempotency: gwdomain.IdempotencyMeta{
			Present: true,
			Outcome: gwdomain.IdempotencyNew,
		},
	}, nil
}

func TestA2AContract_HeadersBackfillTimeoutAndIdempotency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &a2aContractStub{}
	h := NewHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"a2a:call"})
		c.Set(string(types.CtxKeyAuthMethod), "api_key")
		c.Next()
	})
	h.Register(r.Group(""))

	req := httptest.NewRequest(
		http.MethodPost,
		"/a2a/call",
		strings.NewReader(`{"tool_name":"echo","input":{"msg":"ok"}}`),
	)
	req.Header.Set("Idempotency-Key", "idem-123")
	req.Header.Set("X-Timeout-Ms", "2500")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	if svc.lastReq.TimeoutMS != 2500 {
		t.Fatalf("expected timeout from header, got %d", svc.lastReq.TimeoutMS)
	}
	if svc.lastReq.IdempotencyKey == nil || *svc.lastReq.IdempotencyKey != "idem-123" {
		t.Fatalf("expected idempotency key from header, got %#v", svc.lastReq.IdempotencyKey)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out["tool_name"] != "echo" {
		t.Fatalf("expected echo response, got %v", out["tool_name"])
	}
}
