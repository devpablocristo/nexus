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

type cloudProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewCloudProvider(cfg Config) Provider {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.CloudBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &cloudProvider{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(cfg.CloudAPIKey),
		client:  &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *cloudProvider) Generate(ctx context.Context, req ProviderRequest) (map[string]any, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, types.NewHTTPError(400, types.ErrCodeValidation, "cloud llm api key is required")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, types.NewHTTPError(400, types.ErrCodeValidation, "cloud llm model is required")
	}

	body := map[string]any{
		"model": model,
		"messages": []any{
			map[string]any{
				"role":    "system",
				"content": "Return strict JSON only. Do not include markdown fences.",
			},
			map[string]any{
				"role":    "user",
				"content": "task=" + req.Task + " input=" + pretty(req.Input),
			},
		},
		"temperature": 0.1,
		"response_format": map[string]any{
			"type": "json_object",
		},
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, types.NewHTTPError(503, types.ErrCodeNetworkError, "cloud llm unavailable")
	}
	defer resp.Body.Close()
	respRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 400 {
		return nil, types.NewHTTPError(502, types.ErrCodeUpstream5xx, "cloud llm returned error")
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respRaw, &out); err != nil {
		return nil, types.NewHTTPError(502, types.ErrCodeValidation, "invalid cloud llm response")
	}
	if len(out.Choices) == 0 {
		return nil, types.NewHTTPError(502, types.ErrCodeValidation, "cloud llm returned no choices")
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return nil, types.NewHTTPError(502, types.ErrCodeValidation, "cloud llm returned empty content")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, types.NewHTTPError(502, types.ErrCodeValidation, "cloud llm output is not valid json")
	}
	return parsed, nil
}
