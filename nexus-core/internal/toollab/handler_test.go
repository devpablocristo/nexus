package toollab

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

	domain "nexus-core/internal/toollab/usecases/domain"
)

type mockRepo struct {
	fp         string
	snapshots  map[string]string
	schemaResp *domain.SchemaResponse
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		fp:        "sha256:testfp",
		snapshots: map[string]string{},
		schemaResp: &domain.SchemaResponse{
			Database: domain.DatabaseInfo{
				Type:       "postgres",
				Version:    "16",
				SchemaName: "public",
			},
			Entities: []domain.EntityInfo{
				{
					Name:  "tools",
					Table: "tools",
					Columns: []domain.ColumnInfo{
						{Name: "id", Type: "uuid", Nullable: false, PK: true},
						{Name: "name", Type: "text", Nullable: false},
					},
				},
			},
		},
	}
}

func (m *mockRepo) Fingerprint(context.Context) (string, error) { return m.fp, nil }
func (m *mockRepo) CreateSavepoint(_ context.Context, id string) error {
	m.snapshots[id] = m.fp
	return nil
}
func (m *mockRepo) RollbackToSavepoint(context.Context, string) error { return nil }
func (m *mockRepo) TruncateAll(context.Context) error                 { return nil }
func (m *mockRepo) Schema(context.Context) (*domain.SchemaResponse, error) {
	return m.schemaResp, nil
}

func TestHandler_ExposesToollabStandardEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	openapiSpec := []byte(`openapi: "3.0.3"
info:
  title: Nexus API
  version: "1.0.0"
paths:
  /healthz:
    get:
      operationId: healthz
      summary: Health check
  /v1/run:
    post:
      operationId: runTool
      summary: Execute gateway run
`)
	tmp := t.TempDir()
	openapiPath := filepath.Join(tmp, "openapi.yaml")
	if err := os.WriteFile(openapiPath, openapiSpec, 0o644); err != nil {
		t.Fatalf("write openapi fixture: %v", err)
	}

	svc := NewService(newMockRepo(), Config{
		AppVersion:  "1.2.3",
		Environment: "test",
		OpenAPIPath: openapiPath,
	})
	h := NewHandler(svc)

	r := gin.New()
	group := r.Group("/_toollab")
	h.Register(group)

	t.Run("manifest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/_toollab/manifest", nil)
		req.Host = "nexus.example:8080"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("manifest status=%d body=%s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode manifest: %v", err)
		}
		if out["standard_version"] != "1.1" {
			t.Fatalf("expected standard_version=1.1, got %v", out["standard_version"])
		}
		caps, ok := out["capabilities"].([]any)
		if !ok || len(caps) == 0 {
			t.Fatalf("manifest capabilities missing")
		}
	})

	t.Run("profile_and_children", func(t *testing.T) {
		paths := []string{
			"/_toollab/profile",
			"/_toollab/schema",
			"/_toollab/suggested_flows",
			"/_toollab/invariants",
			"/_toollab/limits",
			"/_toollab/environment",
		}
		for _, path := range paths {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Host = "nexus.example:8080"
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
			}
			if ctype := rec.Header().Get("Content-Type"); !strings.Contains(ctype, "application/json") {
				t.Fatalf("%s content-type=%s", path, ctype)
			}
		}
	})

	t.Run("openapi", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/_toollab/openapi", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("openapi status=%d body=%s", rec.Code, rec.Body.String())
		}
		if ctype := rec.Header().Get("Content-Type"); !strings.Contains(ctype, "application/yaml") {
			t.Fatalf("openapi content-type=%s", ctype)
		}
		if !strings.Contains(rec.Body.String(), `openapi: "3.0.3"`) {
			t.Fatalf("unexpected openapi body")
		}
	})

	t.Run("legacy_state_and_metrics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/_toollab/state/fingerprint", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("state/fingerprint status=%d body=%s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/_toollab/metrics", nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("metrics status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
