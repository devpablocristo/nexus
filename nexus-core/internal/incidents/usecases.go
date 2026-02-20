package incidents

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	eventdomain "nexus-core/internal/events/usecases/domain"
	incidentdomain "nexus-core/internal/incidents/usecases/domain"
	"nexus-core/pkg/types"
)

type RepositoryPort interface {
	Create(ctx context.Context, in incidentdomain.Incident) (incidentdomain.Incident, error)
	List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]incidentdomain.Incident, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (incidentdomain.Incident, error)
	Close(ctx context.Context, orgID, id uuid.UUID) (incidentdomain.Incident, error)
}

type EventSink interface {
	Append(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error)
}

type Service interface {
	Create(ctx context.Context, orgID uuid.UUID, actor *string, req CreateRequest) (incidentdomain.Incident, error)
	List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]incidentdomain.Incident, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (incidentdomain.Incident, error)
	Close(ctx context.Context, orgID, id uuid.UUID, actor *string) (incidentdomain.Incident, error)
}

type CreateRequest struct {
	Severity         string
	Title            string
	Summary          string
	RelatedActionIDs []string
	EvidenceRefs     []string
}

type service struct {
	repo   RepositoryPort
	events EventSink
}

func NewService(repo RepositoryPort, events EventSink) Service {
	return &service{repo: repo, events: events}
}

func (s *service) Create(ctx context.Context, orgID uuid.UUID, actor *string, req CreateRequest) (incidentdomain.Incident, error) {
	sev := incidentdomain.Severity(req.Severity)
	switch sev {
	case incidentdomain.SeverityLow, incidentdomain.SeverityMed, incidentdomain.SeverityHigh, incidentdomain.SeverityCrit:
	default:
		return incidentdomain.Incident{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid severity")
	}
	if req.Title == "" || req.Summary == "" {
		return incidentdomain.Incident{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "title and summary required")
	}
	created, err := s.repo.Create(ctx, incidentdomain.Incident{
		OrgID:            orgID,
		Severity:         sev,
		Status:           incidentdomain.StatusOpen,
		Title:            req.Title,
		Summary:          req.Summary,
		RelatedActionIDs: req.RelatedActionIDs,
		EvidenceRefs:     req.EvidenceRefs,
		CreatedBy:        actor,
	})
	if err != nil {
		return incidentdomain.Incident{}, err
	}
	if s.events != nil {
		_, _ = s.events.Append(ctx, orgID, "incident.opened", map[string]any{
			"incident_id":         created.ID.String(),
			"severity":            string(created.Severity),
			"status":              string(created.Status),
			"title":               created.Title,
			"related_action_ids":  created.RelatedActionIDs,
			"evidence_refs":       created.EvidenceRefs,
			"created_by":          created.CreatedBy,
			"opened_at":           created.OpenedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return created, nil
}

func (s *service) List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]incidentdomain.Incident, error) {
	return s.repo.List(ctx, orgID, status, limit)
}

func (s *service) GetByID(ctx context.Context, orgID, id uuid.UUID) (incidentdomain.Incident, error) {
	return s.repo.GetByID(ctx, orgID, id)
}

func (s *service) Close(ctx context.Context, orgID, id uuid.UUID, actor *string) (incidentdomain.Incident, error) {
	current, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return incidentdomain.Incident{}, err
	}
	if current.Status == incidentdomain.StatusClosed {
		return current, nil
	}
	closed, err := s.repo.Close(ctx, orgID, id)
	if err != nil {
		return incidentdomain.Incident{}, err
	}
	if s.events != nil {
		_, _ = s.events.Append(ctx, orgID, "incident.closed", map[string]any{
			"incident_id": closed.ID.String(),
			"closed_by":   actor,
			"closed_at":   closed.ClosedAt,
		})
	}
	return closed, nil
}
