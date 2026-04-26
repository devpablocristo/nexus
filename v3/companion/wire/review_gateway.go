package wire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/governance/go/reviewclient"
)

type reviewGateway struct {
	client  *reviewclient.Client
	baseURL string
	apiKey  string
	http    *http.Client
}

type reviewError struct {
	Kind       string
	StatusCode int
	Body       string
}

func (e reviewError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("review %s: status %d", e.Kind, e.StatusCode)
	}
	return fmt.Sprintf("review %s: status %d body %s", e.Kind, e.StatusCode, e.Body)
}

func newReviewGateway(baseURL, apiKey string) *reviewGateway {
	return &reviewGateway{
		client:  reviewclient.NewClient(baseURL, apiKey),
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *reviewGateway) SubmitRequest(ctx context.Context, idempotencyKey string, body reviewclient.SubmitRequestBody) (reviewclient.SubmitResponse, error) {
	return g.client.SubmitRequest(ctx, idempotencyKey, body)
}

func (g *reviewGateway) GetRequest(ctx context.Context, id string) (reviewclient.RequestSummary, int, error) {
	return g.client.GetRequest(ctx, id)
}

func (g *reviewGateway) GetRequestMeta(ctx context.Context, id string) (status string, orgID string, httpStatus int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/v1/requests/"+id, nil)
	if err != nil {
		return "", "", 0, fmt.Errorf("build review get request: %w", err)
	}
	req.Header.Set("X-API-Key", g.apiKey)
	resp, err := g.http.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("get review request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", resp.StatusCode, nil
	}
	var body struct {
		Status string `json:"status"`
		OrgID  string `json:"org_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", resp.StatusCode, fmt.Errorf("decode review get response: %w", err)
	}
	return body.Status, body.OrgID, resp.StatusCode, nil
}

func (g *reviewGateway) ReportResult(ctx context.Context, id string, success bool, result map[string]any, durationMS int64, errorMessage string) (int, error) {
	payload := map[string]any{
		"success":     success,
		"result":      result,
		"duration_ms": durationMS,
	}
	if strings.TrimSpace(errorMessage) != "" {
		payload["error_message"] = errorMessage
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal review result: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/v1/requests/"+id+"/result", bytes.NewReader(raw))
	if err != nil {
		return 0, fmt.Errorf("build review result request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", g.apiKey)
	resp, err := g.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post review result: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func (g *reviewGateway) CreateAttestation(ctx context.Context, id string, payload map[string]any) (int, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal review attestation: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/v1/requests/"+id+"/attest", bytes.NewReader(raw))
	if err != nil {
		return 0, fmt.Errorf("build review attestation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", g.apiKey)
	resp, err := g.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post review attestation: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return resp.StatusCode, buildReviewError(resp)
	}
	return resp.StatusCode, nil
}

func (g *reviewGateway) GetEvidencePack(ctx context.Context, id string) (map[string]any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/v1/requests/"+id+"/evidence", nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build review evidence request: %w", err)
	}
	req.Header.Set("X-API-Key", g.apiKey)
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("get review evidence: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, resp.StatusCode, buildReviewError(resp)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode review evidence: %w", err)
	}
	return out, resp.StatusCode, nil
}

func buildReviewError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return reviewError{
		Kind:       classifyReviewStatus(resp.StatusCode),
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(raw)),
	}
}

func classifyReviewStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusConflict:
		return "conflict"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "validation"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "unavailable"
	default:
		if status >= 500 {
			return "unavailable"
		}
		return "unexpected"
	}
}
