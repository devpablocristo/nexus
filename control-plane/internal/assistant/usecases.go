package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"nexus/pkg/types"
)

type Config struct {
	OperatorBaseURL string
	OperatorAPIKey  string
	Timeout         time.Duration
}

type Response struct {
	Summary string   `json:"summary"`
	Tables  []Table  `json:"tables"`
	Actions []Action `json:"actions"`
}

type Table struct {
	Title   string              `json:"title"`
	Columns []string            `json:"columns"`
	Rows    []map[string]string `json:"rows"`
}

type Action struct {
	Label      string                 `json:"label"`
	ActionType string                 `json:"action_type"`
	Payload    map[string]interface{} `json:"payload"`
}

type Usecases struct {
	httpClient *http.Client
	cfg        Config
}

func NewUsecases(cfg Config) *Usecases {
	t := cfg.Timeout
	if t <= 0 {
		t = 6 * time.Second
	}
	return &Usecases{
		httpClient: &http.Client{Timeout: t},
		cfg:        cfg,
	}
}

func (u *Usecases) Query(ctx context.Context, orgID uuid.UUID, actor *string, query string) (Response, error) {
	if strings.TrimSpace(query) == "" {
		return Response{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "query is required")
	}
	if strings.TrimSpace(u.cfg.OperatorBaseURL) == "" {
		return Response{}, types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeNetworkError, "operator endpoint is not configured")
	}

	payload := map[string]interface{}{
		"org_id": orgID.String(),
		"query":  query,
		"actor":  actor,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(u.cfg.OperatorBaseURL, "/")+"/v1/assistant/query", bytes.NewReader(body))
	if err != nil {
		return Response{}, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeInternal, "failed to build operator request")
	}
	req.Header.Set("Content-Type", "application/json")
	if u.cfg.OperatorAPIKey != "" {
		req.Header.Set("X-Operator-Key", u.cfg.OperatorAPIKey)
	}
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return Response{}, types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeNetworkError, "operator is unavailable")
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 400 {
		return Response{}, types.NewHTTPError(http.StatusBadGateway, types.ErrCodeUpstream5xx, "operator query failed")
	}
	var out struct {
		Summary string   `json:"summary"`
		Tables  []Table  `json:"tables"`
		Actions []Action `json:"actions"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, types.NewHTTPError(http.StatusBadGateway, types.ErrCodeValidation, "operator returned invalid response")
	}
	return Response{Summary: out.Summary, Tables: out.Tables, Actions: out.Actions}, nil
}

func (u *Usecases) Tick(ctx context.Context) error {
	if strings.TrimSpace(u.cfg.OperatorBaseURL) == "" {
		return types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeNetworkError, "operator endpoint is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(u.cfg.OperatorBaseURL, "/")+"/v1/internal/tick", bytes.NewReader([]byte("{}")))
	if err != nil {
		return types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeInternal, "failed to build operator request")
	}
	req.Header.Set("Content-Type", "application/json")
	if u.cfg.OperatorAPIKey != "" {
		req.Header.Set("X-Operator-Key", u.cfg.OperatorAPIKey)
	}
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeNetworkError, "operator is unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return types.NewHTTPError(http.StatusBadGateway, types.ErrCodeUpstream5xx, "operator tick failed")
	}
	return nil
}
