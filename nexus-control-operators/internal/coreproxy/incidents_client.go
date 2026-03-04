package coreproxy

import (
	"context"
	"time"

	"github.com/google/uuid"

	"nexus-control-operators/internal/incidents"
	incidentdomain "nexus-control-operators/internal/incidents/usecases/domain"
)

type IncidentsClient struct {
	client *Client
}

func NewIncidentsClient(client *Client) *IncidentsClient {
	return &IncidentsClient{client: client}
}

func (c *IncidentsClient) Create(ctx context.Context, orgID uuid.UUID, actor *string, req incidents.CreateRequest) (incidentdomain.Incident, error) {
	body := map[string]any{
		"severity":      req.Severity,
		"title":         req.Title,
		"summary":       req.Summary,
		"evidence_refs": req.EvidenceRefs,
	}
	var resp struct {
		ID        string `json:"id"`
		Severity  string `json:"severity"`
		Status    string `json:"status"`
		Title     string `json:"title"`
		Summary   string `json:"summary"`
		CreatedAt string `json:"created_at"`
	}
	if err := c.client.DoJSON(ctx, "POST", "/internal/operators/incidents", body, &resp); err != nil {
		return incidentdomain.Incident{}, err
	}
	id, _ := uuid.Parse(resp.ID)
	createdAt, _ := time.Parse(time.RFC3339, resp.CreatedAt)
	return incidentdomain.Incident{
		ID:        id,
		OrgID:     orgID,
		Severity:  incidentdomain.Severity(resp.Severity),
		Status:    incidentdomain.Status(resp.Status),
		Title:     resp.Title,
		Summary:   resp.Summary,
		OpenedAt:  createdAt,
		CreatedBy: actor,
	}, nil
}
