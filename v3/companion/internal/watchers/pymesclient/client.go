// Package pymesclient implementa el cliente HTTP a Pymes Core API.
package pymesclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	domain "github.com/devpablocristo/nexus/v3/companion/internal/watchers/usecases/domain"
)

// Client es un cliente HTTP para Pymes Core API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient crea un nuevo cliente para Pymes Core.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) doGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request GET %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response GET %s: %w", path, err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pymes api GET %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c *Client) doPost(ctx context.Context, path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal POST %s: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create request POST %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response POST %s: %w", path, err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pymes api POST %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func parseItems(data []byte) ([]domain.PymesItem, error) {
	var wrapper struct {
		Items []domain.PymesItem `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse items: %w", err)
	}
	return wrapper.Items, nil
}

// GetStaleWorkOrders consulta OTs que llevan más de thresholdDays sin avanzar.
func (c *Client) GetStaleWorkOrders(ctx context.Context, orgID string, thresholdDays int) ([]domain.PymesItem, error) {
	path := fmt.Sprintf("/v1/work-orders?status=in_progress&stale_days=%d", thresholdDays)
	data, err := c.doGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get stale work orders: %w", err)
	}
	return parseItems(data)
}

// GetUnconfirmedAppointments consulta turnos no confirmados para mañana.
func (c *Client) GetUnconfirmedAppointments(ctx context.Context, orgID string, hoursBefore int) ([]domain.PymesItem, error) {
	path := "/v1/appointments?confirmed=false&upcoming_hours=" + fmt.Sprintf("%d", hoursBefore)
	data, err := c.doGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get unconfirmed appointments: %w", err)
	}
	return parseItems(data)
}

// GetLowStockItems consulta productos con stock bajo.
func (c *Client) GetLowStockItems(ctx context.Context, orgID string, thresholdUnits int) ([]domain.PymesItem, error) {
	path := fmt.Sprintf("/v1/inventory/low-stock?threshold=%d", thresholdUnits)
	data, err := c.doGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get low stock items: %w", err)
	}
	return parseItems(data)
}

// GetInactiveCustomers consulta clientes que no visitan hace más de thresholdMonths.
func (c *Client) GetInactiveCustomers(ctx context.Context, orgID string, thresholdMonths int) ([]domain.PymesItem, error) {
	path := fmt.Sprintf("/v1/customers?inactive_months=%d", thresholdMonths)
	data, err := c.doGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get inactive customers: %w", err)
	}
	return parseItems(data)
}

// GetRevenueComparison consulta comparación de facturación mensual.
func (c *Client) GetRevenueComparison(ctx context.Context, orgID string) (*domain.RevenueComparison, error) {
	data, err := c.doGet(ctx, "/v1/dashboard/revenue?compare=previous_month")
	if err != nil {
		return nil, fmt.Errorf("get revenue comparison: %w", err)
	}
	var result domain.RevenueComparison
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse revenue comparison: %w", err)
	}
	return &result, nil
}

// SendWhatsAppTemplate envía un template de WhatsApp.
func (c *Client) SendWhatsAppTemplate(ctx context.Context, orgID, partyID, templateName string, params map[string]string) error {
	payload := map[string]any{
		"party_id":      partyID,
		"template_name": templateName,
		"language":      "es",
		"params":        params,
	}
	_, err := c.doPost(ctx, "/v1/whatsapp/send/template", payload)
	if err != nil {
		return fmt.Errorf("send whatsapp template: %w", err)
	}
	return nil
}

// SendWhatsAppText envía un mensaje de texto por WhatsApp.
func (c *Client) SendWhatsAppText(ctx context.Context, orgID, partyID, body string) error {
	payload := map[string]any{
		"party_id": partyID,
		"body":     body,
	}
	_, err := c.doPost(ctx, "/v1/whatsapp/send/text", payload)
	if err != nil {
		return fmt.Errorf("send whatsapp text: %w", err)
	}
	return nil
}
