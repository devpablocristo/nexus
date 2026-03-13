package saasclient

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"data-plane/internal/gateway"
)

type ProtectedResourcesClient struct {
	baseURL     string
	internalKey string
	client      *http.Client
	log         zerolog.Logger
}

func NewProtectedResourcesClient(log zerolog.Logger) *ProtectedResourcesClient {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXUS_SAAS_URL")), "/")
	key := strings.TrimSpace(os.Getenv("NEXUS_SAAS_INTERNAL_KEY"))
	timeoutMS := 300
	if raw := strings.TrimSpace(os.Getenv("NEXUS_SAAS_TIMEOUT_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			timeoutMS = parsed
		}
	}
	return &ProtectedResourcesClient{
		baseURL:     baseURL,
		internalKey: key,
		client:      &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
		log:         log,
	}
}

type protectedResourcesResponse struct {
	Items []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ResourceType string `json:"resource_type"`
		MatchValue   string `json:"match_value"`
		MatchMode    string `json:"match_mode"`
		Environment  string `json:"environment"`
		Reason       string `json:"reason"`
	} `json:"items"`
}

func (c *ProtectedResourcesClient) ListProtectedResources(ctx context.Context, orgID uuid.UUID) ([]gateway.ProtectedResource, error) {
	if c.baseURL == "" || c.internalKey == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/protected-resources/"+orgID.String(), nil)
	if err != nil {
		return nil, nil
	}
	req.Header.Set("X-NEXUS-SAAS-KEY", c.internalKey)
	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Warn().Err(err).Msg("saas protected resources request failed")
		return nil, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		c.log.Warn().Int("status_code", resp.StatusCode).Msg("saas protected resources non-success response")
		return nil, nil
	}
	var out protectedResourcesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.log.Warn().Err(err).Msg("saas protected resources decode failed")
		return nil, nil
	}
	items := make([]gateway.ProtectedResource, 0, len(out.Items))
	for _, item := range out.Items {
		id, _ := uuid.Parse(item.ID)
		items = append(items, gateway.ProtectedResource{
			ID:           id,
			Name:         item.Name,
			ResourceType: item.ResourceType,
			MatchValue:   item.MatchValue,
			MatchMode:    item.MatchMode,
			Environment:  item.Environment,
			Reason:       item.Reason,
		})
	}
	return items, nil
}
