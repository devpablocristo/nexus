package usagemetering

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	CounterAPICalls = "api_calls"
)

// MeteringPort is the narrow interface consumed by integration points.
// Each consumer package declares its own copy (hexagonal pattern).
type MeteringPort interface {
	Increment(ctx context.Context, orgID uuid.UUID, counter string) error
}

type Repository struct {
	baseURL     string
	internalKey string
	client      *http.Client
}

func NewRepository(_ *gorm.DB) *Repository {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXUS_SAAS_URL")), "/")
	key := strings.TrimSpace(os.Getenv("NEXUS_SAAS_INTERNAL_KEY"))
	timeoutMS := 300
	if raw := strings.TrimSpace(os.Getenv("NEXUS_SAAS_TIMEOUT_MS")); raw != "" {
		if v, err := time.ParseDuration(raw + "ms"); err == nil && v > 0 {
			timeoutMS = int(v / time.Millisecond)
		}
	}
	return &Repository{
		baseURL:     baseURL,
		internalKey: key,
		client:      &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
	}
}

type usageEventRequest struct {
	EventID string `json:"event_id"`
	OrgID   string `json:"org_id"`
	Counter string `json:"counter"`
}

// Increment forwards usage events to nexus-saas.
// It is best-effort: callers are expected to ignore failures.
func (r *Repository) Increment(ctx context.Context, orgID uuid.UUID, counter string) error {
	if r.baseURL == "" || r.internalKey == "" {
		return nil
	}
	payload := usageEventRequest{
		EventID: uuid.NewString(),
		OrgID:   orgID.String(),
		Counter: counter,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/internal/usage/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-NEXUS-SAAS-KEY", r.internalKey)
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("saas usage ingestion failed with status %d", resp.StatusCode)
	}
	return nil
}
