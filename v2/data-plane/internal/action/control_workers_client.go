package action

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

type ControlWorkersClient struct {
	baseURL string
	client  *http.Client
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
