package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdomain "nexus-gateway/internal/gateway/usecases/domain"
	mcpdto "nexus-gateway/internal/mcp/handler/dto"
	tooldomain "nexus-gateway/internal/tool/usecases/domain"
	"nexus-gateway/pkg/types"
)

type mcpSvcStub struct{ called bool }

func (s *mcpSvcStub) ListTools(context.Context, uuid.UUID) ([]tooldomain.Tool, error) {
	s.called = true
	return []tooldomain.Tool{}, nil
}
func (s *mcpSvcStub) GetTool(context.Context, uuid.UUID, string) (tooldomain.Tool, error) {
	s.called = true
	return tooldomain.Tool{}, nil
}
func (s *mcpSvcStub) CallTool(context.Context, uuid.UUID, gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	s.called = true
	return gwdomain.RunResponse{HTTPStatus: http.StatusOK, Status: gwdomain.RunStatusSuccess, Decision: gwdomain.DecisionAllow}, nil
}

func TestMCPListAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mcpSvcStub{}
	h := NewHandler(svc)
	payload := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"tools:read"})
		c.Next()
	})
	h.Register(r.Group(""))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(payload)))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 envelope got %d", w.Code)
	}
	var out mcpdto.JSONRPCResponse
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Error == nil {
		t.Fatalf("expected rpc error")
	}
	if code, _ := out.Error.Data["error_code"].(string); code != types.ErrCodeUnauthorized {
		t.Fatalf("expected unauthorized error code got %v", out.Error.Data)
	}

	r = gin.New()
	svc.called = false
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"mcp:read"})
		c.Next()
	})
	h.Register(r.Group(""))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(payload)))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !svc.called {
		t.Fatalf("expected service call")
	}
}
