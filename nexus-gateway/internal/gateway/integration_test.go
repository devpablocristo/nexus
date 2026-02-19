package gateway_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"nexus-gateway/cmd/config"
	auditdto "nexus-gateway/internal/audit/handler/dto"
	"nexus-gateway/internal/org"
	"nexus-gateway/internal/policy"
	"nexus-gateway/internal/tool"
	"nexus-gateway/pkg/utils"
	"nexus-gateway/pkg/validations/jsonschema"
	"nexus-gateway/wire"
)

func TestIntegration_RunTransferPoliciesAndAuditRedaction(t *testing.T) {
	ctx := context.Background()

	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_DB":       "nexus",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}
	pg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	host, err := pg.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://postgres:postgres@%s:%s/nexus?sslmode=disable", host, port.Port())

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	migDir := filepath.Join(projectRoot(t), "migrations")
	migrator := utils.Migrator{DB: sqlDB, Dir: migDir}
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/transfer":
			var body struct {
				Amount float64 `json:"amount"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Amount <= 0 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_AMOUNT", "message": "amount must be > 0"}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "tx_id": "tx_123", "amount": body.Amount})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mock.Close)

	gdb, err := gorm.Open(gormpg.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}

	orgR := org.NewRepository(gdb)
	toolR := tool.NewRepository(gdb)
	policyR := policy.NewRepository(gdb)

	schemaCache := jsonschema.NewCompilerCache()
	toolSvc := tool.NewService(toolR, nil, schemaCache)
	policySvc := policy.NewService(policyR, toolSvc)

	orgID, err := orgR.UpsertOrgByName(ctx, "demo")
	if err != nil {
		t.Fatalf("org upsert: %v", err)
	}
	apiKey := "integration-demo-key"
	apiHash := utils.SHA256Hex(apiKey)
	if err := orgR.UpsertAPIKey(ctx, orgID, apiHash, "demo-key"); err != nil {
		t.Fatalf("apikey upsert: %v", err)
	}

	_, err = toolSvc.Create(ctx, orgID, tool.CreateRequest{
		Name:        "transfer",
		Kind:        "http",
		Method:      "POST",
		URL:         mock.URL + "/transfer",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"amount": map[string]any{"type": "number"}}, "required": []any{"amount"}},
		ActionType:  "write",
		RiskLevel:   3,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("tool create: %v", err)
	}

	_, _ = policySvc.CreateForTool(ctx, orgID, "transfer", policy.CreateRequest{
		Effect:         "deny",
		Priority:       10,
		Conditions:     map[string]any{"path": "input.amount", "op": "gt", "value": 1000},
		Limits:         map[string]any{},
		ReasonTemplate: "Denied because amount too high",
		Enabled:        true,
	})
	_, _ = policySvc.CreateForTool(ctx, orgID, "transfer", policy.CreateRequest{
		Effect:   "allow",
		Priority: 20,
		Conditions: map[string]any{
			"all": []any{
				map[string]any{"path": "input.amount", "op": "lte", "value": 1000},
				map[string]any{"path": "context.user_id", "op": "exists"},
			},
		},
		Limits:         map[string]any{},
		ReasonTemplate: "Allowed",
		Enabled:        true,
	})

	app, cleanup, err := wire.InitializeAPI(config.Config{
		API:        config.APIConfig{HTTPPort: 0},
		DB:         config.DBConfig{DatabaseURL: dsn},
		HTTPServer: config.HTTPServerConfig{MaxBodyBytes: 262144},
		Migrations: config.MigrationsConfig{Dir: migDir},
		Service: config.ServiceConfig{
			LogLevel:               "disabled",
			SwaggerCDN:             true,
			HTTPTimeoutMS:          2000,
			HTTPMaxResponseBytes:   1048576,
			RateLimitDefaultPerMin: 60,
			MasterKey:              "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			DisableSSRFProtection:  true,
		},
	})
	if err != nil {
		t.Fatalf("init api: %v", err)
	}
	t.Cleanup(cleanup)

	// Add egress rule for mock server host (required by default-deny policy).
	mockURL, _ := url.Parse(mock.URL)
	egressBody, _ := json.Marshal(map[string]any{"host": mockURL.Hostname()})
	egressReq := httptest.NewRequest(http.MethodPost, "/v1/tools/transfer/egress-rules", bytes.NewReader(egressBody))
	egressReq.Header.Set("Content-Type", "application/json")
	egressReq.Header.Set("X-NEXUS-GATEWAY-KEY", apiKey)
	egressRR := httptest.NewRecorder()
	app.Router.ServeHTTP(egressRR, egressReq)
	if egressRR.Code != http.StatusNoContent {
		t.Fatalf("egress rule: expected 204 got %d body=%s", egressRR.Code, egressRR.Body.String())
	}

	doRun := func(body any) *httptest.ResponseRecorder {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-NEXUS-GATEWAY-KEY", apiKey)
		rr := httptest.NewRecorder()
		app.Router.ServeHTTP(rr, req)
		return rr
	}

	rr1 := doRun(map[string]any{
		"tool_name": "transfer",
		"input":     map[string]any{"amount": 5000, "token": "secret"},
		"context":   map[string]any{"user_id": "u1"},
	})
	if rr1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d body=%s", rr1.Code, rr1.Body.String())
	}

	rr2 := doRun(map[string]any{
		"tool_name": "transfer",
		"input":     map[string]any{"amount": 500, "token": "secret"},
		"context":   map[string]any{},
	})
	if rr2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d body=%s", rr2.Code, rr2.Body.String())
	}

	rr3 := doRun(map[string]any{
		"tool_name": "transfer",
		"input":     map[string]any{"amount": 500, "card_number": "4111111111111111"},
		"context":   map[string]any{"user_id": "u1", "token": "secret"},
	})
	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr3.Code, rr3.Body.String())
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/v1/audit?tool_name=transfer&limit=5", nil)
	auditReq.Header.Set("X-NEXUS-GATEWAY-KEY", apiKey)
	rr := httptest.NewRecorder()
	app.Router.ServeHTTP(rr, auditReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("audit expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var auditRes auditdto.ListAuditResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &auditRes); err != nil {
		t.Fatalf("audit json: %v", err)
	}
	if len(auditRes.Items) == 0 {
		t.Fatalf("expected audit items")
	}
	// Ensure redaction applied.
	found := false
	for _, it := range auditRes.Items {
		if m, ok := it.Input.(map[string]any); ok {
			if v, ok := m["token"]; ok && v == "***" {
				found = true
			}
			if v, ok := m["card_number"]; ok && v == "***" {
				found = true
			}
		}
		if m, ok := it.Context.(map[string]any); ok {
			if v, ok := m["token"]; ok && v == "***" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected redacted fields in audit")
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/v1/audit/export?format=jsonl&tool_name=transfer&limit=5", nil)
	exportReq.Header.Set("X-NEXUS-GATEWAY-KEY", apiKey)
	exportRR := httptest.NewRecorder()
	app.Router.ServeHTTP(exportRR, exportReq)
	if exportRR.Code != http.StatusOK {
		t.Fatalf("export expected 200 got %d body=%s", exportRR.Code, exportRR.Body.String())
	}
	lines := bytes.Split(bytes.TrimSpace(exportRR.Body.Bytes()), []byte("\n"))
	if len(lines) == 0 {
		t.Fatalf("expected export lines")
	}
	var first map[string]any
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatalf("jsonl decode: %v", err)
	}
	if _, ok := first["event_hash"]; !ok {
		t.Fatalf("expected event_hash in export line: %v", first)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// go test runs in package dir; walk up until go.mod.
	dir := wd
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not find project root from %s", wd)
	return ""
}
