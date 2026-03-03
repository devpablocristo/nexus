package world

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"nexus-core/pkg/types"
)

const (
	internalKeyHeader = "X-Sim-Engine-Internal-Key"
	requestIDHeader   = "X-Nexus-Request-Id"
)

type Config struct {
	BaseURL     string
	InternalKey string
	Timeout     time.Duration
}

type Usecases struct {
	client *http.Client
	cfg    Config
}

func NewUsecases(cfg Config) *Usecases {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	return &Usecases{
		client: &http.Client{Timeout: timeout},
		cfg:    cfg,
	}
}

func (u *Usecases) ListRuns(ctx context.Context, orgID uuid.UUID, requestID string, limit int, cursor string) (any, error) {
	q := url.Values{}
	q.Set("org_id", orgID.String())
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if strings.TrimSpace(cursor) != "" {
		q.Set("cursor", cursor)
	}
	return u.call(ctx, requestID, http.MethodGet, "/admin/run/runs", q, nil)
}

func (u *Usecases) GetState(ctx context.Context, orgID uuid.UUID, requestID, runID string, stepID *int64) (any, error) {
	q := url.Values{}
	q.Set("org_id", orgID.String())
	q.Set("run_id", runID)
	if stepID != nil {
		q.Set("step_id", strconv.FormatInt(*stepID, 10))
	}
	return u.call(ctx, requestID, http.MethodGet, "/admin/run/state", q, nil)
}

func (u *Usecases) GetEvents(ctx context.Context, orgID uuid.UUID, requestID, runID string, fromSeq int64, limit int) (any, error) {
	q := url.Values{}
	q.Set("org_id", orgID.String())
	q.Set("run_id", runID)
	if fromSeq > 0 {
		q.Set("from_seq", strconv.FormatInt(fromSeq, 10))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return u.call(ctx, requestID, http.MethodGet, "/admin/run/events", q, nil)
}

func (u *Usecases) CreateRun(ctx context.Context, orgID uuid.UUID, requestID string, payload map[string]any) (any, error) {
	body := cloneMap(payload)
	body["org_id"] = orgID.String()
	return u.call(ctx, requestID, http.MethodPost, "/admin/run/create", nil, body)
}

func (u *Usecases) Replay(ctx context.Context, orgID uuid.UUID, requestID string, payload map[string]any) (any, error) {
	body := cloneMap(payload)
	body["org_id"] = orgID.String()
	return u.call(ctx, requestID, http.MethodPost, "/admin/run/replay", nil, body)
}

func (u *Usecases) call(ctx context.Context, requestID, method, path string, query url.Values, payload map[string]any) (any, error) {
	base := strings.TrimRight(strings.TrimSpace(u.cfg.BaseURL), "/")
	if base == "" {
		return nil, types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeNetworkError, "sim-engine endpoint is not configured")
	}
	fullURL := base + path
	if query != nil && len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid request payload")
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeInternal, "failed to build sim-engine request")
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set(requestIDHeader, requestID)
	}
	if strings.TrimSpace(u.cfg.InternalKey) != "" {
		req.Header.Set(internalKeyHeader, u.cfg.InternalKey)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeNetworkError, "sim-engine unavailable")
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 400 {
		return nil, mapUpstreamError(resp.StatusCode, raw)
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, types.NewHTTPError(http.StatusBadGateway, types.ErrCodeValidation, "sim-engine returned invalid json")
	}
	return out, nil
}

func mapUpstreamError(status int, raw []byte) error {
	msg := "sim-engine request failed"
	code := types.ErrCodeNetworkError
	httpStatus := http.StatusBadGateway

	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		code = types.ErrCodeUnauthorized
		httpStatus = status
	case http.StatusBadRequest:
		code = types.ErrCodeValidation
		httpStatus = http.StatusBadRequest
	case http.StatusNotFound:
		code = types.ErrCodeNotFound
		httpStatus = http.StatusNotFound
	case http.StatusTooManyRequests:
		code = types.ErrCodeRateLimited
		httpStatus = http.StatusTooManyRequests
	default:
		if status >= 500 {
			code = types.ErrCodeUpstream5xx
		}
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		if e, ok := parsed["error"].(map[string]any); ok {
			if s, ok := e["message"].(string); ok && strings.TrimSpace(s) != "" {
				msg = s
			}
		}
		if s, ok := parsed["message"].(string); ok && strings.TrimSpace(s) != "" {
			msg = s
		}
	}
	return types.NewHTTPError(httpStatus, code, msg)
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
