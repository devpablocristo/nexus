package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sharedapikey "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/apikey"
	sharedobservability "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/observability"
)

// Actor describes who triggered the audited event.
type Actor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// WriteRequest is the shared write contract for audit ingestion.
type WriteRequest struct {
	EventType     string         `json:"event_type"`
	SourceService string         `json:"source_service"`
	ActionID      string         `json:"action_id,omitempty"`
	IncidentID    string         `json:"incident_id,omitempty"`
	AlertID       string         `json:"alert_id,omitempty"`
	ResourceID    string         `json:"resource_id,omitempty"`
	ResourceType  string         `json:"resource_type,omitempty"`
	Actor         *Actor         `json:"actor,omitempty"`
	Summary       string         `json:"summary"`
	Data          map[string]any `json:"data,omitempty"`
	OccurredAt    time.Time      `json:"occurred_at,omitempty"`
}

// Client writes audit records into control-plane.
type Client struct {
	baseURL string
	client  *http.Client
	apiKey  string
}

// NewClient builds a control-plane audit client.
func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

// WithAPIKey configures outbound API key auth for control-plane writes.
func (c *Client) WithAPIKey(key string) *Client {
	if c == nil {
		return nil
	}
	c.apiKey = strings.TrimSpace(key)
	return c
}

// Create sends one audit record to control-plane.
func (c *Client) Create(ctx context.Context, req WriteRequest) error {
	if c == nil || c.baseURL == "" {
		return nil
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal audit payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/audit", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build audit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	sharedapikey.Apply(httpReq, c.apiKey)
	sharedobservability.ApplyRequestID(httpReq, ctx)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("audit write returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
