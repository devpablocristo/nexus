package audit

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	auditdomain "nexus/v2/control-plane/internal/audit/usecases/domain"
)

type CreateRequest = sharedaudit.WriteRequest

type ListRequest struct {
	ActionID   string
	IncidentID string
	AlertID    string
	ResourceID string
	ActorID    string
	EventType  string
	From       time.Time
	To         time.Time
	Limit      int
}

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e httpError) Error() string { return e.Message }

func newHTTPError(status int, code, message string) error {
	return httpError{Status: status, Code: code, Message: message}
}

type Usecases struct {
	repo Repository
}

func NewUsecases(repo Repository) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (auditdomain.AuditRecord, error) {
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		return auditdomain.AuditRecord{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "event_type required")
	}
	sourceService := strings.TrimSpace(req.SourceService)
	if sourceService == "" {
		return auditdomain.AuditRecord{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "source_service required")
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return auditdomain.AuditRecord{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "summary required")
	}
	if req.Actor != nil {
		if strings.TrimSpace(req.Actor.Type) == "" || strings.TrimSpace(req.Actor.ID) == "" {
			return auditdomain.AuditRecord{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "actor.type and actor.id required when actor is present")
		}
	}

	record := auditdomain.AuditRecord{
		EventType:     eventType,
		SourceService: sourceService,
		ActionID:      trimOrEmpty(req.ActionID),
		IncidentID:    trimOrEmpty(req.IncidentID),
		AlertID:       trimOrEmpty(req.AlertID),
		ResourceID:    trimOrEmpty(req.ResourceID),
		ResourceType:  trimOrEmpty(req.ResourceType),
		Actor:         req.Actor,
		Summary:       summary,
		Data:          cloneData(req.Data),
		OccurredAt:    req.OccurredAt.UTC(),
	}
	if record.OccurredAt.IsZero() {
		record.OccurredAt = nowUTC()
	}
	return u.repo.Create(ctx, record)
}

func (u *Usecases) List(ctx context.Context, req ListRequest) ([]auditdomain.AuditRecord, error) {
	if !req.From.IsZero() && !req.To.IsZero() && req.From.After(req.To) {
		return nil, newHTTPError(http.StatusBadRequest, "VALIDATION", "from must be <= to")
	}
	limit := req.Limit
	switch {
	case limit <= 0:
		limit = 50
	case limit > 200:
		limit = 200
	}
	return u.repo.List(ctx, ListFilters{
		ActionID:   trimOrEmpty(req.ActionID),
		IncidentID: trimOrEmpty(req.IncidentID),
		AlertID:    trimOrEmpty(req.AlertID),
		ResourceID: trimOrEmpty(req.ResourceID),
		ActorID:    trimOrEmpty(req.ActorID),
		EventType:  trimOrEmpty(req.EventType),
		From:       req.From.UTC(),
		To:         req.To.UTC(),
		Limit:      limit,
	})
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (auditdomain.AuditRecord, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return auditdomain.AuditRecord{}, mapRepoErr(err)
	}
	return item, nil
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "audit record not found")
	}
	return err
}
