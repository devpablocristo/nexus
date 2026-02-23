package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCloudProvider_RequiresAPIKey(t *testing.T) {
	t.Parallel()
	p := NewCloudProvider(Config{
		CloudBaseURL: "https://api.openai.com/v1",
		CloudAPIKey:  "",
		Timeout:      2 * time.Second,
	})
	if _, err := p.Generate(context.Background(), ProviderRequest{
		Task:  "diagnosis",
		Model: "gpt-4o-mini",
		Input: map[string]any{"org_id": "org-1"},
	}); err == nil {
		t.Fatalf("expected missing api key error")
	}
}

func TestCloudProvider_ParsesOpenAIStyleResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatalf("expected authorization header")
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": `{"answer":"ok","evidence_refs":["incident:1"]}`,
					},
				},
			},
		})
	}))
	defer srv.Close()

	p := NewCloudProvider(Config{
		CloudBaseURL: srv.URL,
		CloudAPIKey:  "test-key",
		Timeout:      2 * time.Second,
	})
	out, err := p.Generate(context.Background(), ProviderRequest{
		Task:  "executive_qa",
		Model: "gpt-4o-mini",
		Input: map[string]any{"org_id": "org-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["answer"] != "ok" {
		t.Fatalf("unexpected parsed payload: %+v", out)
	}
}
