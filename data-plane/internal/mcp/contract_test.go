package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdomain "data-plane/internal/gateway/usecases/domain"
	mcpdto "data-plane/internal/mcp/handler/dto"
	tooldomain "data-plane/internal/tool/usecases/domain"
	"nexus/pkg/types"
)

type mcpContractStub struct {
	lastRun gwdomain.RunRequest
}

func (s *mcpContractStub) ListTools(context.Context, uuid.UUID) ([]tooldomain.Tool, error) {
	return []tooldomain.Tool{}, nil
}

func (s *mcpContractStub) GetTool(context.Context, uuid.UUID, string) (tooldomain.Tool, error) {
	return tooldomain.Tool{}, nil
}

func (s *mcpContractStub) CallTool(context.Context, uuid.UUID, gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	return gwdomain.RunResponse{
		RequestID:  "req-1",
		Decision:   gwdomain.DecisionAllow,
		ToolName:   "echo",
		Status:     gwdomain.RunStatusSuccess,
		HTTPStatus: http.StatusOK,
	}, nil
}

func TestMCPContract_InvalidRequestReturnsJSONRPCError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mcpContractStub{}
	h := NewHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.New())
		c.Set(string(types.CtxKeyScopes), []string{"mcp:read"})
		c.Next()
	})
	h.Register(r.Group(""))

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
	var out mcpdto.JSONRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out.Error == nil {
		t.Fatalf("expected rpc error")
	}
	if code, _ := out.Error.Data["error_code"].(string); code != types.ErrCodeValidation {
		t.Fatalf("expected validation error code got %v", out.Error.Data)
	}
}

func TestMCPContract_ErrorCatalogIncludesCoreCodes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "pkgs", "contracts", "error-codes.json"))
	if err != nil {
		t.Fatalf("read contract file: %v", err)
	}
	var doc struct {
		Codes []string `json:"codes"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal contract file: %v", err)
	}
	required := map[string]struct{}{
		types.ErrCodeValidation:       {},
		types.ErrCodeUnauthorized:     {},
		types.ErrCodeNotFound:         {},
		types.ErrCodeApprovalRequired:    {},
		types.ErrCodeIdempotencyRequired: {},
		types.ErrCodeRateLimited:         {},
	}
	for _, code := range doc.Codes {
		delete(required, code)
	}
	if len(required) > 0 {
		t.Fatalf("missing error codes in shared catalog: %v", required)
	}
}
