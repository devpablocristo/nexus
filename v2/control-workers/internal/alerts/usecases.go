package alerts

import (
	"context"
	"errors"
	"net/http"
	"strings"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	"github.com/google/uuid"

	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
)

type CreateRequest struct {
	SourceKind   alertdomain.SourceKind
	SourceID     string
	ActionID     string
	ResourceID   string
	ResourceType string
	Channel      alertdomain.Channel
	Route        string
	Severity     alertdomain.Severity
	Status       alertdomain.Status
	Summary      string
	Body         string
	Details      map[string]any
}

type UpdateRequest struct {
	Status  *alertdomain.Status
	Summary *string
	Body    *string
	Details map[string]any
}

type ListRequest struct {
	SourceKind string
	Channel    string
	Severity   string
	Status     string
	Archived   *bool
	Limit      int
}

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e httpError) Error() string {
	return e.Message
}

func newHTTPError(status int, code, message string) error {
	return httpError{Status: status, Code: code, Message: message}
}

type Usecases struct {
	repo    Repository
	audit   AuditSink
	metrics MetricsSink
}

func NewUsecases(repo Repository) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (alertdomain.Alert, error) {
	normalized, err := normalizeCreate(req)
	if err != nil {
		return alertdomain.Alert{}, err
	}
	created, err := u.repo.Create(ctx, normalized)
	if err != nil {
		return alertdomain.Alert{}, err
	}
	if u.metrics != nil {
		u.metrics.IncAlertCreated(string(created.Channel), string(created.Severity))
	}
	actionID := detailStringValue(created.Details, "action_id")
	resourceID := detailStringValue(created.Details, "resource_id")
	resourceType := detailStringValue(created.Details, "resource_type")
	incidentID := ""
	if created.SourceKind == alertdomain.SourceKindIncident {
		incidentID = created.SourceID
	}
	u.emitAudit(ctx, sharedaudit.WriteRequest{
		EventType:     "alert_created",
		SourceService: "control-workers",
		ActionID:      actionID,
		IncidentID:    incidentID,
		AlertID:       created.ID,
		ResourceID:    resourceID,
		ResourceType:  resourceType,
		Actor:         &sharedaudit.Actor{Type: "system", ID: "control-workers"},
		Summary:       "alert created",
		Data: map[string]any{
			"incident_id":   incidentID,
			"action_id":     actionID,
			"alert_id":      created.ID,
			"source_kind":   string(created.SourceKind),
			"source_id":     created.SourceID,
			"resource_id":   resourceID,
			"resource_type": resourceType,
			"channel":       string(created.Channel),
			"route":         created.Route,
			"severity":      string(created.Severity),
			"status":        string(created.Status),
		},
		OccurredAt: created.CreatedAt,
	})
	return created, nil
}

func (u *Usecases) List(ctx context.Context, req ListRequest) ([]alertdomain.Alert, error) {
	archived := req.Archived
	if archived == nil {
		defaultArchived := false
		archived = &defaultArchived
	}

	filters := ListFilters{
		SourceKind: strings.TrimSpace(req.SourceKind),
		Channel:    strings.TrimSpace(req.Channel),
		Severity:   strings.TrimSpace(req.Severity),
		Status:     strings.TrimSpace(req.Status),
		Archived:   archived,
		Limit:      req.Limit,
	}
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.SourceKind != "" {
		if err := validateSourceKind(alertdomain.SourceKind(filters.SourceKind)); err != nil {
			return nil, err
		}
	}
	if filters.Channel != "" {
		if err := validateChannel(alertdomain.Channel(filters.Channel)); err != nil {
			return nil, err
		}
	}
	if filters.Severity != "" {
		if err := validateSeverity(alertdomain.Severity(filters.Severity)); err != nil {
			return nil, err
		}
	}
	if filters.Status != "" {
		if err := validateStatus(alertdomain.Status(filters.Status)); err != nil {
			return nil, err
		}
	}
	return u.repo.List(ctx, filters)
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return alertdomain.Alert{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) UpdateByID(ctx context.Context, id uuid.UUID, req UpdateRequest) (alertdomain.Alert, error) {
	current, err := u.GetByID(ctx, id)
	if err != nil {
		return alertdomain.Alert{}, err
	}

	if req.Status != nil {
		if err := validateStatus(*req.Status); err != nil {
			return alertdomain.Alert{}, err
		}
		current.Status = *req.Status
	}
	if req.Summary != nil {
		summary := strings.TrimSpace(*req.Summary)
		if summary == "" {
			return alertdomain.Alert{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "summary required")
		}
		current.Summary = summary
	}
	if req.Body != nil {
		body := strings.TrimSpace(*req.Body)
		if body == "" {
			return alertdomain.Alert{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "body required")
		}
		current.Body = body
	}
	if req.Details != nil {
		current.Details = cloneDetails(req.Details)
	}

	updated, err := u.repo.Update(ctx, current)
	if err != nil {
		return alertdomain.Alert{}, mapRepoErr(err)
	}
	return updated, nil
}

func (u *Usecases) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return mapRepoErr(u.repo.Delete(ctx, id))
}

func (u *Usecases) ArchiveByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error) {
	item, err := u.repo.Archive(ctx, id, nowUTC())
	if err != nil {
		return alertdomain.Alert{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) RestoreByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error) {
	item, err := u.repo.Restore(ctx, id, nowUTC())
	if err != nil {
		return alertdomain.Alert{}, mapRepoErr(err)
	}
	return item, nil
}

type Sink interface {
	Create(ctx context.Context, req CreateRequest) (alertdomain.Alert, error)
}

func normalizeCreate(req CreateRequest) (alertdomain.Alert, error) {
	if err := validateSourceKind(req.SourceKind); err != nil {
		return alertdomain.Alert{}, err
	}
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return alertdomain.Alert{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "source_id required")
	}
	if err := validateChannel(req.Channel); err != nil {
		return alertdomain.Alert{}, err
	}
	route := strings.TrimSpace(req.Route)
	if route == "" {
		return alertdomain.Alert{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "route required")
	}
	if err := validateSeverity(req.Severity); err != nil {
		return alertdomain.Alert{}, err
	}
	status := req.Status
	if status == "" {
		status = alertdomain.StatusPending
	}
	if err := validateStatus(status); err != nil {
		return alertdomain.Alert{}, err
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return alertdomain.Alert{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "summary required")
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return alertdomain.Alert{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "body required")
	}
	details := cloneDetails(req.Details)
	if details == nil {
		details = make(map[string]any, 4)
	}
	if req.SourceKind == alertdomain.SourceKindIncident {
		details["incident_id"] = sourceID
	}
	if actionID := strings.TrimSpace(req.ActionID); actionID != "" {
		details["action_id"] = actionID
	}
	if resourceID := strings.TrimSpace(req.ResourceID); resourceID != "" {
		details["resource_id"] = resourceID
	}
	if resourceType := strings.TrimSpace(req.ResourceType); resourceType != "" {
		details["resource_type"] = resourceType
	}

	return alertdomain.Alert{
		SourceKind: req.SourceKind,
		SourceID:   sourceID,
		Channel:    req.Channel,
		Route:      route,
		Severity:   req.Severity,
		Status:     status,
		Summary:    summary,
		Body:       body,
		Details:    details,
	}, nil
}

func validateSourceKind(value alertdomain.SourceKind) error {
	switch value {
	case alertdomain.SourceKindIncident:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported source_kind")
	}
}

func validateChannel(value alertdomain.Channel) error {
	switch value {
	case alertdomain.ChannelSlack, alertdomain.ChannelPagerDuty, alertdomain.ChannelEmail:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported channel")
	}
}

func validateSeverity(value alertdomain.Severity) error {
	switch value {
	case alertdomain.SeverityLow, alertdomain.SeverityMedium, alertdomain.SeverityHigh, alertdomain.SeverityCritical:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported severity")
	}
}

func validateStatus(value alertdomain.Status) error {
	switch value {
	case alertdomain.StatusPending, alertdomain.StatusDispatched, alertdomain.StatusSuppressed, alertdomain.StatusAcknowledged:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported status")
	}
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "alert not found")
	}
	if errors.Is(err, ErrArchived) {
		return newHTTPError(http.StatusConflict, "ARCHIVED", "alert is archived")
	}
	if errors.Is(err, ErrAlreadyArchived) {
		return newHTTPError(http.StatusConflict, "ALREADY_ARCHIVED", "alert already archived")
	}
	if errors.Is(err, ErrNotArchived) {
		return newHTTPError(http.StatusConflict, "NOT_ARCHIVED", "alert is not archived")
	}
	return err
}

func detailStringValue(details map[string]any, key string) string {
	if len(details) == 0 {
		return ""
	}
	raw, ok := details[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
