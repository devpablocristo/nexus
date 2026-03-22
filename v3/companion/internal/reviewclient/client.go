package reviewclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	corehttp "github.com/devpablocristo/core/backend/go/httpclient"
	"github.com/google/uuid"
)

// Client HTTP mínimo hacia Review (governance core).
type Client struct {
	caller *corehttp.Caller
}

// NewClient crea el cliente; baseURL sin slash final.
func NewClient(baseURL, apiKey string) *Client {
	h := make(http.Header)
	h.Set("X-API-Key", apiKey)
	return &Client{
		caller: &corehttp.Caller{
			BaseURL: baseURL,
			Header:  h,
			HTTP: &http.Client{
				Timeout: 30 * time.Second,
			},
		},
	}
}

// SubmitRequestBody alinea con review requests SubmitRequest.
type SubmitRequestBody struct {
	RequesterType  string         `json:"requester_type"`
	RequesterID    string         `json:"requester_id"`
	RequesterName  string         `json:"requester_name,omitempty"`
	ActionType     string         `json:"action_type"`
	TargetSystem   string         `json:"target_system,omitempty"`
	TargetResource string         `json:"target_resource,omitempty"`
	Params         map[string]any `json:"params,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	Context        string         `json:"context,omitempty"`
}

// SubmitResponse alinea con review SubmitResponse.
type SubmitResponse struct {
	RequestID      string `json:"request_id"`
	Decision       string `json:"decision"`
	RiskLevel      string `json:"risk_level"`
	DecisionReason string `json:"decision_reason"`
	Status         string `json:"status"`
}

// RequestSummary respuesta de GET /v1/requests/{id} (subset para UI).
type RequestSummary struct {
	ID             string `json:"id"`
	RequesterType  string `json:"requester_type"`
	RequesterID    string `json:"requester_id"`
	ActionType     string `json:"action_type"`
	TargetSystem   string `json:"target_system"`
	TargetResource string `json:"target_resource"`
	Reason         string `json:"reason"`
	RiskLevel      string `json:"risk_level"`
	Decision       string `json:"decision"`
	DecisionReason string `json:"decision_reason"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SubmitRequest POST /v1/requests
func (c *Client) SubmitRequest(ctx context.Context, idempotencyKey string, body SubmitRequestBody) (SubmitResponse, error) {
	var out SubmitResponse
	// Caller no expone cabeceras por request; duplicamos una petición con Idempotency-Key vía http directo si hace falta.
	// Para mantener simplicidad, usamos DoJSON y si idempotencyKey != "", inyectamos en caller temporal — mejor: método dedicado.
	if idempotencyKey != "" {
		return c.submitWithIdempotency(ctx, idempotencyKey, body)
	}
	st, raw, err := c.caller.DoJSON(ctx, http.MethodPost, "/v1/requests", body)
	if err != nil {
		return out, err
	}
	if st != http.StatusCreated {
		return out, fmt.Errorf("review submit: status %d body %s", st, string(raw))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode submit response: %w", err)
	}
	return out, nil
}

func (c *Client) submitWithIdempotency(ctx context.Context, idempotencyKey string, body SubmitRequestBody) (SubmitResponse, error) {
	var out SubmitResponse
	// Reutiliza Caller.DoJSON añadiendo header: clonamos Header y añadimos Idempotency-Key en una petición one-off.
	h := make(http.Header)
	for k, vv := range c.caller.Header {
		for _, v := range vv {
			h.Add(k, v)
		}
	}
	h.Set("Idempotency-Key", idempotencyKey)
	one := &corehttp.Caller{
		HTTP:    c.caller.HTTP,
		BaseURL: c.caller.BaseURL,
		Header:  h,
	}
	st, raw, err := one.DoJSON(ctx, http.MethodPost, "/v1/requests", body)
	if err != nil {
		return out, err
	}
	if st != http.StatusCreated {
		return out, fmt.Errorf("review submit: status %d body %s", st, string(raw))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode submit response: %w", err)
	}
	return out, nil
}

// GetRequest GET /v1/requests/{id}
func (c *Client) GetRequest(ctx context.Context, id uuid.UUID) (RequestSummary, int, error) {
	var out RequestSummary
	path := "/v1/requests/" + id.String()
	st, raw, err := c.caller.DoJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return out, 0, err
	}
	if st == http.StatusNotFound {
		return out, st, nil
	}
	if st != http.StatusOK {
		return out, st, fmt.Errorf("review get request: status %d body %s", st, string(raw))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, st, fmt.Errorf("decode get response: %w", err)
	}
	return out, st, nil
}

// ParseErrorBody intenta extraer mensaje de error de Review.
func ParseErrorBody(raw []byte) string {
	var eb errorBody
	if json.Unmarshal(raw, &eb) == nil && eb.Message != "" {
		return eb.Message
	}
	return string(raw)
}
