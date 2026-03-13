package nexus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a lightweight Nexus Core SDK client.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewClient returns a client with sane defaults.
func NewClient(baseURL, apiKey string) *Client {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  strings.TrimSpace(apiKey),
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RunRequest is the request payload for /v1/run.
type RunRequest struct {
	ToolName string            `json:"tool_name"`
	Input    map[string]any    `json:"input"`
	Context  map[string]any    `json:"context,omitempty"`
	Timeout  int               `json:"timeout_ms,omitempty"`
	Headers  map[string]string `json:"-"`
}

// RunResponse contains the gateway execution result.
type RunResponse struct {
	RequestID string         `json:"request_id"`
	Decision  string         `json:"decision"`
	ToolName  string         `json:"tool_name"`
	Status    string         `json:"status"`
	Result    map[string]any `json:"result,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	Error     *APIError      `json:"error,omitempty"`
	LatencyMS int            `json:"latency_ms"`
}

// Tool is the minimal shape returned by /v1/tools.
type Tool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	ActionType  string `json:"action_type"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
}

type listToolsResponse struct {
	Items []Tool `json:"items"`
}

type apiErrorEnvelope struct {
	Error APIError `json:"error"`
}

// APIError mirrors Nexus HTTP error payloads.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e APIError) Error() string {
	if strings.TrimSpace(e.Code) == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// RunTool executes a registered tool through the gateway.
func (c *Client) RunTool(ctx context.Context, req RunRequest) (*RunResponse, error) {
	if strings.TrimSpace(req.ToolName) == "" {
		return nil, fmt.Errorf("tool_name is required")
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	var out RunResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/run", req, &out, req.Headers); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTools returns registered tools.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var out listToolsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/tools", nil, &out, nil); err != nil {
		return nil, err
	}
	if out.Items == nil {
		return []Tool{}, nil
	}
	return out.Items, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any, extraHeaders map[string]string) error {
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		payload = bytes.NewReader(raw)
	}

	url := c.BaseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("X-NEXUS-CORE-KEY", c.APIKey)
	}
	for k, v := range extraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		rawBody, _ := io.ReadAll(resp.Body)
		var envelope apiErrorEnvelope
		if json.Unmarshal(rawBody, &envelope) == nil && strings.TrimSpace(envelope.Error.Message) != "" {
			return fmt.Errorf("http %d: %w", resp.StatusCode, envelope.Error)
		}
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
