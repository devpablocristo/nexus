package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
)

type ControlWorkersClient struct {
	baseURL string
	client  *http.Client
	apiKey  string
}

func NewControlWorkersClient(baseURL string, timeout time.Duration) *ControlWorkersClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &ControlWorkersClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *ControlWorkersClient) WithAPIKey(key string) *ControlWorkersClient {
	if c == nil {
		return nil
	}
	c.apiKey = strings.TrimSpace(key)
	return c
}

func (c *ControlWorkersClient) Create(ctx context.Context, req IncidentRequest) error {
	if c.baseURL == "" {
		return nil
	}

	payload := map[string]any{
		"source_kind":   "action",
		"source_id":     req.SourceID,
		"action_type":   string(req.ActionType),
		"resource_id":   req.ResourceID,
		"resource_type": string(req.ResourceType),
		"trigger":       string(req.Trigger),
		"risk_level":    string(req.RiskLevel),
		"summary":       req.Summary,
		"reason":        req.Reason,
		"details":       cloneMap(req.Details),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal incident payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/incidents", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build control-workers incident request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	sharedapikey.Apply(httpReq, c.apiKey)
	sharedobservability.ApplyRequestID(httpReq, ctx)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("create control-workers incident: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("control-workers incident returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (c *ControlWorkersClient) CountOpenByResource(ctx context.Context, resourceID string) (int, error) {
	if c.baseURL == "" || strings.TrimSpace(resourceID) == "" {
		return 0, nil
	}

	endpoint, err := url.Parse(c.baseURL + "/v1/incidents")
	if err != nil {
		return 0, fmt.Errorf("build control-workers incidents url: %w", err)
	}
	query := endpoint.Query()
	query.Set("resource_id", resourceID)
	query.Set("status", "open")
	query.Set("archived", "false")
	query.Set("limit", "50")
	endpoint.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("build control-workers incident list request: %w", err)
	}
	sharedapikey.Apply(httpReq, c.apiKey)
	sharedobservability.ApplyRequestID(httpReq, ctx)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("list control-workers incidents: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("control-workers incident list returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var payload struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("decode control-workers incidents: %w", err)
	}
	return len(payload.Items), nil
}
