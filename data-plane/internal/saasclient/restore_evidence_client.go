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

type RestoreEvidenceClient struct {
	baseURL     string
	internalKey string
	client      *http.Client
	log         zerolog.Logger
}

func NewRestoreEvidenceClient(log zerolog.Logger) *RestoreEvidenceClient {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXUS_SAAS_URL")), "/")
	key := strings.TrimSpace(os.Getenv("NEXUS_SAAS_INTERNAL_KEY"))
	timeoutMS := 300
	if raw := strings.TrimSpace(os.Getenv("NEXUS_SAAS_TIMEOUT_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			timeoutMS = parsed
		}
	}
	return &RestoreEvidenceClient{
		baseURL:     baseURL,
		internalKey: key,
		client:      &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
		log:         log,
	}
}

type restoreEvidenceResponse struct {
	Items []struct {
		ID             string         `json:"id"`
		Environment    string         `json:"environment"`
		System         string         `json:"system"`
		Status         string         `json:"status"`
		SnapshotID     string         `json:"snapshot_id"`
		RestoreTarget  string         `json:"restore_target"`
		StartedAt      string         `json:"started_at"`
		CompletedAt    string         `json:"completed_at"`
		Source         string         `json:"source"`
		ArtifactSHA256 string         `json:"artifact_sha256"`
		Summary        map[string]any `json:"summary"`
		CreatedAt      string         `json:"created_at"`
	} `json:"items"`
}

func (c *RestoreEvidenceClient) ListRestoreEvidence(ctx context.Context, orgID uuid.UUID, environment string, limit int) ([]gateway.RestoreEvidence, error) {
	if c.baseURL == "" || c.internalKey == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/restore-evidence/"+orgID.String(), nil)
	if err != nil {
		return nil, nil
	}
	q := req.URL.Query()
	if env := strings.TrimSpace(environment); env != "" {
		q.Set("environment", env)
	}
	q.Set("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-NEXUS-SAAS-KEY", c.internalKey)
	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Warn().Err(err).Msg("saas restore evidence request failed")
		return nil, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		c.log.Warn().Int("status_code", resp.StatusCode).Msg("saas restore evidence non-success response")
		return nil, nil
	}
	var out restoreEvidenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.log.Warn().Err(err).Msg("saas restore evidence decode failed")
		return nil, nil
	}
	items := make([]gateway.RestoreEvidence, 0, len(out.Items))
	for _, item := range out.Items {
		id, _ := uuid.Parse(item.ID)
		items = append(items, gateway.RestoreEvidence{
			ID:             id,
			Environment:    item.Environment,
			System:         item.System,
			Status:         item.Status,
			SnapshotID:     item.SnapshotID,
			RestoreTarget:  item.RestoreTarget,
			StartedAt:      parseRFC3339Ptr(item.StartedAt),
			CompletedAt:    parseRFC3339Ptr(item.CompletedAt),
			Source:         item.Source,
			ArtifactSHA256: item.ArtifactSHA256,
			Summary:        item.Summary,
			CreatedAt:      parseRFC3339(item.CreatedAt),
		})
	}
	return items, nil
}

func parseRFC3339Ptr(raw string) *time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func parseRFC3339(raw string) time.Time {
	value := parseRFC3339Ptr(raw)
	if value == nil {
		return time.Time{}
	}
	return *value
}
