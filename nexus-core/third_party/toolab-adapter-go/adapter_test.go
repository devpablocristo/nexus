package toolab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManifestMinimal(t *testing.T) {
	a := NewAdapter(Config{AppName: "test-app", AppVersion: "1.0.0"})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/manifest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["adapter_version"] != "1" {
		t.Errorf("expected adapter_version 1, got %v", resp["adapter_version"])
	}
	if resp["app_name"] != "test-app" {
		t.Errorf("expected app_name test-app, got %v", resp["app_name"])
	}
	caps, ok := resp["capabilities"].([]any)
	if !ok {
		t.Fatalf("expected capabilities array, got %T", resp["capabilities"])
	}
	if len(caps) != 0 {
		t.Errorf("expected 0 capabilities for minimal config, got %d", len(caps))
	}
}

func TestManifestWithProviders(t *testing.T) {
	a := NewAdapter(Config{
		AppName:         "test-app",
		AppVersion:      "2.0.0",
		MetricsProvider: &mockMetrics{},
		LogsProvider:    &mockLogs{},
		SeedProvider:    &mockSeed{},
	})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/manifest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	caps := resp["capabilities"].([]any)

	expected := map[string]bool{"seed": false, "metrics": false, "logs": false}
	for _, c := range caps {
		expected[c.(string)] = true
	}
	for cap, found := range expected {
		if !found {
			t.Errorf("expected capability %q not found", cap)
		}
	}
}

func TestMetricsEndpoint(t *testing.T) {
	a := NewAdapter(Config{
		AppName:         "test-app",
		AppVersion:      "1.0.0",
		MetricsProvider: &mockMetrics{},
	})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	metrics := resp["metrics"].([]any)
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(metrics))
	}
}

func TestLogsEndpoint(t *testing.T) {
	a := NewAdapter(Config{
		AppName:      "test-app",
		AppVersion:   "1.0.0",
		LogsProvider: &mockLogs{},
	})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/logs?limit=10", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	lines := resp["lines"].([]any)
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
}

func TestSeedApplyAndClear(t *testing.T) {
	a := NewAdapter(Config{
		AppName:      "test-app",
		AppVersion:   "1.0.0",
		SeedProvider: &mockSeed{},
	})
	handler := a.Handler()

	// Apply seed
	body := `{"run_seed":"42","scope":["uuid","timestamp"]}`
	req := httptest.NewRequest(http.MethodPost, "/seed", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["applied"] != true {
		t.Error("expected applied=true")
	}

	// Clear seed
	req = httptest.NewRequest(http.MethodDelete, "/seed", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["cleared"] != true {
		t.Error("expected cleared=true")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	a := NewAdapter(Config{AppName: "test-app", AppVersion: "1.0.0"})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodPost, "/manifest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestStandardManifestAndDiscoveryEndpoints(t *testing.T) {
	a := NewAdapter(Config{
		AppName:                "test-app",
		AppVersion:             "3.0.0",
		BaseURL:                "http://localhost:8080",
		SchemaProvider:         &mockSchemaProvider{},
		SuggestedFlowsProvider: &mockSuggestedFlowsProvider{},
		InvariantsProvider:     &mockInvariantsProvider{},
		LimitsProvider:         &mockLimitsProvider{},
		EnvironmentProvider:    &mockEnvironmentProvider{},
		OpenAPIProvider:        &mockOpenAPIProvider{},
	})
	handler := a.Handler()

	t.Run("manifest includes standard fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/manifest", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("manifest status=%d body=%s", w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode manifest: %v", err)
		}
		if resp["standard_version"] != "1.1" {
			t.Fatalf("expected standard_version=1.1, got %v", resp["standard_version"])
		}
		links, ok := resp["links"].(map[string]any)
		if !ok || links["profile_url"] == nil {
			t.Fatalf("expected links.profile_url in manifest")
		}
	})

	t.Run("discovery endpoints", func(t *testing.T) {
		endpoints := []string{
			"/profile",
			"/schema",
			"/suggested_flows",
			"/invariants",
			"/limits",
			"/environment",
		}
		for _, path := range endpoints {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("%s invalid content-type: %s", path, w.Header().Get("Content-Type"))
			}
		}
	})

	t.Run("openapi endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("openapi status=%d body=%s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Header().Get("Content-Type"), "application/yaml") {
			t.Fatalf("unexpected openapi content-type: %s", w.Header().Get("Content-Type"))
		}
		if !strings.Contains(w.Body.String(), "openapi: \"3.0.3\"") {
			t.Fatalf("unexpected openapi body: %s", w.Body.String())
		}
	})
}

func TestProfileIncludesUnknownsWhenProviderFails(t *testing.T) {
	a := NewAdapter(Config{
		AppName:        "test-app",
		AppVersion:     "1.0.0",
		SchemaProvider: &mockFailSchemaProvider{},
	})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	unknowns, ok := resp["unknowns"].([]any)
	if !ok || len(unknowns) == 0 {
		t.Fatalf("expected unknowns in profile response")
	}
}

// --- Mocks ---

type mockMetrics struct{}

func (m *mockMetrics) Snapshot(_ context.Context) ([]Metric, error) {
	return []Metric{
		{Name: "requests_total", Type: "counter", Value: 100, Labels: map[string]string{"method": "GET"}},
	}, nil
}

type mockLogs struct{}

func (m *mockLogs) Collect(_ context.Context, _ time.Time, _ int, _ string) ([]LogLine, error) {
	return []LogLine{
		{Timestamp: "2025-01-01T00:00:00Z", Level: "INFO", Message: "test"},
	}, nil
}

type mockSeed struct{}

func (m *mockSeed) Apply(_ context.Context, _ string, scope []string) (SeedResult, error) {
	return SeedResult{Applied: []string{"uuid"}, Ignored: []string{"timestamp"}}, nil
}

func (m *mockSeed) Clear(_ context.Context) error { return nil }

type mockSchemaProvider struct{}

func (m *mockSchemaProvider) Schema(_ context.Context) (any, error) {
	return map[string]any{
		"database": map[string]any{
			"type": "postgres",
		},
		"entities": []map[string]any{
			{
				"name":  "users",
				"table": "users",
				"columns": []map[string]any{
					{"name": "id", "type": "uuid", "nullable": false},
				},
			},
		},
	}, nil
}

type mockFailSchemaProvider struct{}

func (m *mockFailSchemaProvider) Schema(_ context.Context) (any, error) {
	return nil, context.DeadlineExceeded
}

type mockSuggestedFlowsProvider struct{}

func (m *mockSuggestedFlowsProvider) SuggestedFlows(_ context.Context) (any, error) {
	empty := ""
	return map[string]any{
		"flows": []map[string]any{
			{
				"id": "health_flow",
				"requests": []map[string]any{
					{"method": "GET", "path": "/healthz", "body": empty},
				},
			},
		},
	}, nil
}

type mockInvariantsProvider struct{}

func (m *mockInvariantsProvider) Invariants(_ context.Context) (any, error) {
	return map[string]any{
		"invariants": []map[string]any{
			{"id": "inv_no_5xx", "type": "no_5xx_allowed"},
		},
	}, nil
}

type mockLimitsProvider struct{}

func (m *mockLimitsProvider) Limits(_ context.Context) (any, error) {
	return map[string]any{
		"rate": map[string]any{
			"requests_per_second": 10,
		},
	}, nil
}

type mockEnvironmentProvider struct{}

func (m *mockEnvironmentProvider) Environment(_ context.Context) (any, error) {
	return map[string]any{
		"mode":      "test",
		"read_only": false,
		"features":  map[string]bool{"toolab_standard_v1_1": true},
	}, nil
}

type mockOpenAPIProvider struct{}

func (m *mockOpenAPIProvider) OpenAPIDocument(_ context.Context) (string, []byte, error) {
	return "application/yaml", []byte("openapi: \"3.0.3\"\ninfo:\n  title: Test\n  version: \"1.0.0\"\npaths: {}\n"), nil
}

func (m *mockOpenAPIProvider) OpenAPIInfo(_ context.Context) (*OpenAPIInfo, error) {
	return &OpenAPIInfo{
		ContentType: "application/yaml",
		Version:     "3.0.3",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, nil
}
