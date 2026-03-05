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
	"nexus/pkg/types"
)

type EntitlementsClient struct {
	baseURL     string
	internalKey string
	client      *http.Client
	log         zerolog.Logger
}

func NewEntitlementsClient(log zerolog.Logger) *EntitlementsClient {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXUS_SAAS_URL")), "/")
	key := strings.TrimSpace(os.Getenv("NEXUS_SAAS_INTERNAL_KEY"))
	timeoutMS := 300
	if raw := strings.TrimSpace(os.Getenv("NEXUS_SAAS_TIMEOUT_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			timeoutMS = parsed
		}
	}
	return &EntitlementsClient{
		baseURL:     baseURL,
		internalKey: key,
		client:      &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
		log:         log,
	}
}

type entitlementsResponse struct {
	OrgID      string         `json:"org_id"`
	PlanCode   string         `json:"plan_code"`
	Status     string         `json:"status"`
	HardLimits map[string]any `json:"hard_limits"`
}

// GetRunRPM implements gateway.TenantLimitsPort semantics.
// It returns 0 when SaaS is unavailable, so core can fallback to defaults,
// except when tenant lifecycle state is suspended/deleted.
func (c *EntitlementsClient) GetRunRPM(ctx context.Context, orgID uuid.UUID) (int, error) {
	if c.baseURL == "" || c.internalKey == "" {
		return 0, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/entitlements/"+orgID.String(), nil)
	if err != nil {
		return 0, nil
	}
	req.Header.Set("X-NEXUS-SAAS-KEY", c.internalKey)
	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Warn().Err(err).Msg("saas entitlements request failed")
		return 0, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		c.log.Warn().Int("status_code", resp.StatusCode).Msg("saas entitlements non-success response")
		return 0, nil
	}
	var out entitlementsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.log.Warn().Err(err).Msg("saas entitlements decode failed")
		return 0, nil
	}
	if out.HardLimits == nil {
		if strings.TrimSpace(out.Status) != "" && !strings.EqualFold(out.Status, "active") {
			return 0, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "tenant suspended/deleted")
		}
		return 0, nil
	}
	if strings.TrimSpace(out.Status) != "" && !strings.EqualFold(out.Status, "active") {
		return 0, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "tenant suspended/deleted")
	}
	switch v := out.HardLimits["run_rpm"].(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, nil
	}
}
