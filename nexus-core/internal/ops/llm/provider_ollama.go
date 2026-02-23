package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"nexus-core/pkg/types"
)

type ollamaProvider struct {
	baseURL string
	client  *http.Client
}

func NewOllamaProvider(cfg Config) Provider {
	baseURL := strings.TrimRight(cfg.OllamaBaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &ollamaProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: cfg.Timeout},
	}
}

func (o *ollamaProvider) Generate(ctx context.Context, req ProviderRequest) (map[string]any, error) {
	body := map[string]any{
		"model":  req.Model,
		"stream": false,
		"format": "json",
		"prompt": "Return only strict JSON for task=" + req.Task + " input=" + pretty(req.Input),
		"options": map[string]any{
			"temperature": 0.1,
		},
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/generate", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, types.NewHTTPError(503, types.ErrCodeNetworkError, "ollama unavailable")
	}
	defer resp.Body.Close()
	respRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 400 {
		return nil, types.NewHTTPError(502, types.ErrCodeUpstream5xx, "ollama returned error")
	}
	var out struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(respRaw, &out); err != nil {
		return nil, types.NewHTTPError(502, types.ErrCodeValidation, "invalid ollama response")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out.Response), &parsed); err != nil {
		return nil, types.NewHTTPError(502, types.ErrCodeValidation, "ollama output is not valid json")
	}
	return parsed, nil
}
