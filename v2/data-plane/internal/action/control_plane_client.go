package action

import (
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

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type ResourceResolver interface {
	GetByID(ctx context.Context, resourceID string) (actiondomain.ProtectedResource, error)
}

type ActionPolicy struct {
	ID                 string
	ActionType         string
	ResourceType       string
	Effect             string
	Priority           int
	Expression         string
	Reason             string
	RequireApproval    bool
	ApprovalTTLSeconds int
	Enabled            bool
}

type PolicySource interface {
	List(ctx context.Context, actionType, resourceType string) ([]ActionPolicy, error)
}

type ControlPlaneClient struct {
	baseURL string
	client  *http.Client
	apiKey  string
}

func NewControlPlaneClient(baseURL string, timeout time.Duration) *ControlPlaneClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &ControlPlaneClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *ControlPlaneClient) WithAPIKey(key string) *ControlPlaneClient {
	if c == nil {
		return nil
	}
	c.apiKey = strings.TrimSpace(key)
	return c
}

func (c *ControlPlaneClient) GetByID(ctx context.Context, resourceID string) (actiondomain.ProtectedResource, error) {
	if c.baseURL == "" {
		return actiondomain.ProtectedResource{}, ErrResourceNotFound
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/resources/"+url.PathEscape(resourceID), nil)
	if err != nil {
		return actiondomain.ProtectedResource{}, fmt.Errorf("build control-plane resource request: %w", err)
	}
	sharedapikey.Apply(req, c.apiKey)
	sharedobservability.ApplyRequestID(req, ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return actiondomain.ProtectedResource{}, fmt.Errorf("fetch control-plane resource: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return actiondomain.ProtectedResource{}, ErrResourceNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return actiondomain.ProtectedResource{}, fmt.Errorf("control-plane resource returned status %d", resp.StatusCode)
	}

	var payload struct {
		ID          string            `json:"id"`
		Type        string            `json:"type"`
		Name        string            `json:"name"`
		Environment string            `json:"environment"`
		Chain       string            `json:"chain"`
		Labels      map[string]string `json:"labels"`
		Criticality string            `json:"criticality"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return actiondomain.ProtectedResource{}, fmt.Errorf("decode control-plane resource: %w", err)
	}

	return actiondomain.ProtectedResource{
		ID:          payload.ID,
		Type:        actiondomain.ResourceType(payload.Type),
		Name:        payload.Name,
		Environment: payload.Environment,
		Chain:       payload.Chain,
		Labels:      cloneLabels(payload.Labels),
		Criticality: payload.Criticality,
	}, nil
}

func (c *ControlPlaneClient) List(ctx context.Context, actionType, resourceType string) ([]ActionPolicy, error) {
	if c.baseURL == "" {
		return nil, nil
	}

	endpoint, err := url.Parse(c.baseURL + "/v1/policies")
	if err != nil {
		return nil, fmt.Errorf("build control-plane policy url: %w", err)
	}
	query := endpoint.Query()
	query.Set("action_type", actionType)
	query.Set("resource_type", resourceType)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build control-plane policy request: %w", err)
	}
	sharedapikey.Apply(req, c.apiKey)
	sharedobservability.ApplyRequestID(req, ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch control-plane policies: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("control-plane policies returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Items []struct {
			ID                 string `json:"id"`
			ActionType         string `json:"action_type"`
			ResourceType       string `json:"resource_type"`
			Effect             string `json:"effect"`
			Priority           int    `json:"priority"`
			Expression         string `json:"expression"`
			Reason             string `json:"reason"`
			RequireApproval    bool   `json:"require_approval"`
			ApprovalTTLSeconds int    `json:"approval_ttl_seconds"`
			Enabled            bool   `json:"enabled"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode control-plane policies: %w", err)
	}

	items := make([]ActionPolicy, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, ActionPolicy{
			ID:                 item.ID,
			ActionType:         item.ActionType,
			ResourceType:       item.ResourceType,
			Effect:             item.Effect,
			Priority:           item.Priority,
			Expression:         item.Expression,
			Reason:             item.Reason,
			RequireApproval:    item.RequireApproval,
			ApprovalTTLSeconds: item.ApprovalTTLSeconds,
			Enabled:            item.Enabled,
		})
	}
	return items, nil
}

func cloneLabels(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
