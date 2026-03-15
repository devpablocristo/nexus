package incidents

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	"github.com/google/uuid"

	"nexus/v2/control-workers/internal/alerts"
	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
	incidentdomain "nexus/v2/control-workers/internal/incidents/usecases/domain"
)

type CreateRequest struct {
	SourceKind   incidentdomain.SourceKind
	SourceID     string
	ActionType   string
	ResourceID   string
	ResourceType string
	Trigger      incidentdomain.Trigger
	RiskLevel    incidentdomain.RiskLevel
	Summary      string
	Reason       string
	Details      map[string]any
}

type UpdateRequest struct {
	Status  *incidentdomain.Status
	Summary *string
	Reason  *string
	Details map[string]any
}

type ListRequest struct {
	SourceKind string
	Trigger    string
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
	repo   Repository
	alerts alerts.Sink
	audit  AuditSink
}

func NewUsecases(repo Repository) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) WithAlertSink(sink alerts.Sink) *Usecases {
	u.alerts = sink
	return u
}

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (incidentdomain.Incident, error) {
	normalized, err := normalizeCreate(req)
	if err != nil {
		return incidentdomain.Incident{}, err
	}
	created, err := u.repo.Create(ctx, normalized)
	if err != nil {
		return incidentdomain.Incident{}, err
	}
	u.emitAudit(ctx, sharedaudit.WriteRequest{
		EventType:     "incident_created",
		SourceService: "control-workers",
		ActionID:      created.SourceID,
		ResourceID:    created.ResourceID,
		ResourceType:  created.ResourceType,
		Actor:         &sharedaudit.Actor{Type: "system", ID: "control-workers"},
		Summary:       "incident created",
		Data: map[string]any{
			"incident_id": created.ID,
			"trigger":     string(created.Trigger),
			"severity":    string(created.Severity),
			"status":      string(created.Status),
			"source_kind": string(created.SourceKind),
		},
		OccurredAt: created.CreatedAt,
	})
	u.emitAlert(ctx, created)
	return created, nil
}

func (u *Usecases) List(ctx context.Context, req ListRequest) ([]incidentdomain.Incident, error) {
	archived := req.Archived
	if archived == nil {
		defaultArchived := false
		archived = &defaultArchived
	}

	filters := ListFilters{
		SourceKind: strings.TrimSpace(req.SourceKind),
		Trigger:    strings.TrimSpace(req.Trigger),
		Severity:   strings.TrimSpace(req.Severity),
		Status:     strings.TrimSpace(req.Status),
		Archived:   archived,
		Limit:      req.Limit,
	}
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.SourceKind != "" {
		if err := validateSourceKind(incidentdomain.SourceKind(filters.SourceKind)); err != nil {
			return nil, err
		}
	}
	if filters.Trigger != "" {
		if err := validateTrigger(incidentdomain.Trigger(filters.Trigger)); err != nil {
			return nil, err
		}
	}
	if filters.Severity != "" {
		if err := validateSeverity(incidentdomain.Severity(filters.Severity)); err != nil {
			return nil, err
		}
	}
	if filters.Status != "" {
		if err := validateStatus(incidentdomain.Status(filters.Status)); err != nil {
			return nil, err
		}
	}
	return u.repo.List(ctx, filters)
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return incidentdomain.Incident{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) UpdateByID(ctx context.Context, id uuid.UUID, req UpdateRequest) (incidentdomain.Incident, error) {
	current, err := u.GetByID(ctx, id)
	if err != nil {
		return incidentdomain.Incident{}, err
	}

	if req.Status != nil {
		if err := validateStatus(*req.Status); err != nil {
			return incidentdomain.Incident{}, err
		}
		current.Status = *req.Status
		switch *req.Status {
		case incidentdomain.StatusResolved:
			now := nowUTC()
			current.ResolvedAt = &now
		default:
			current.ResolvedAt = nil
		}
	}
	if req.Summary != nil {
		summary := strings.TrimSpace(*req.Summary)
		if summary == "" {
			return incidentdomain.Incident{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "summary required")
		}
		current.Summary = summary
	}
	if req.Reason != nil {
		current.Reason = strings.TrimSpace(*req.Reason)
	}
	if req.Details != nil {
		current.Details = cloneDetails(req.Details)
	}

	updated, err := u.repo.Update(ctx, current)
	if err != nil {
		return incidentdomain.Incident{}, mapRepoErr(err)
	}
	return updated, nil
}

func (u *Usecases) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return mapRepoErr(u.repo.Delete(ctx, id))
}

func (u *Usecases) ArchiveByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error) {
	item, err := u.repo.Archive(ctx, id, nowUTC())
	if err != nil {
		return incidentdomain.Incident{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) RestoreByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error) {
	item, err := u.repo.Restore(ctx, id, nowUTC())
	if err != nil {
		return incidentdomain.Incident{}, mapRepoErr(err)
	}
	return item, nil
}

func normalizeCreate(req CreateRequest) (incidentdomain.Incident, error) {
	if err := validateSourceKind(req.SourceKind); err != nil {
		return incidentdomain.Incident{}, err
	}
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return incidentdomain.Incident{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "source_id required")
	}
	actionType := strings.TrimSpace(req.ActionType)
	if actionType == "" {
		return incidentdomain.Incident{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "action_type required")
	}
	resourceID := strings.TrimSpace(req.ResourceID)
	if resourceID == "" {
		return incidentdomain.Incident{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "resource_id required")
	}
	resourceType := strings.TrimSpace(req.ResourceType)
	if resourceType == "" {
		return incidentdomain.Incident{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "resource_type required")
	}
	if err := validateTrigger(req.Trigger); err != nil {
		return incidentdomain.Incident{}, err
	}
	if err := validateRiskLevel(req.RiskLevel); err != nil {
		return incidentdomain.Incident{}, err
	}

	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		summary = deriveSummary(req.Trigger, actionType)
	}

	return incidentdomain.Incident{
		SourceKind:   req.SourceKind,
		SourceID:     sourceID,
		ActionType:   actionType,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Trigger:      req.Trigger,
		RiskLevel:    req.RiskLevel,
		Severity:     deriveSeverity(req.Trigger, req.RiskLevel),
		Status:       incidentdomain.StatusOpen,
		Summary:      summary,
		Reason:       strings.TrimSpace(req.Reason),
		Details:      cloneDetails(req.Details),
	}, nil
}

func deriveSummary(trigger incidentdomain.Trigger, actionType string) string {
	switch trigger {
	case incidentdomain.TriggerBlockedAction:
		return fmt.Sprintf("%s blocked by Nexus", actionType)
	case incidentdomain.TriggerApprovalRejected:
		return fmt.Sprintf("%s rejected during approval", actionType)
	case incidentdomain.TriggerApprovalExpired:
		return fmt.Sprintf("%s approval expired", actionType)
	case incidentdomain.TriggerExecutionFailed:
		return fmt.Sprintf("%s failed during execution", actionType)
	default:
		return fmt.Sprintf("%s requires operator attention", actionType)
	}
}

func deriveSeverity(trigger incidentdomain.Trigger, riskLevel incidentdomain.RiskLevel) incidentdomain.Severity {
	switch trigger {
	case incidentdomain.TriggerExecutionFailed:
		if riskLevel == incidentdomain.RiskLevelHigh || riskLevel == incidentdomain.RiskLevelCritical {
			return incidentdomain.SeverityCritical
		}
		return incidentdomain.SeverityHigh
	case incidentdomain.TriggerBlockedAction, incidentdomain.TriggerApprovalRejected:
		if riskLevel == incidentdomain.RiskLevelCritical {
			return incidentdomain.SeverityCritical
		}
		if riskLevel == incidentdomain.RiskLevelHigh {
			return incidentdomain.SeverityHigh
		}
		return incidentdomain.SeverityMedium
	default:
		if riskLevel == incidentdomain.RiskLevelCritical {
			return incidentdomain.SeverityHigh
		}
		return incidentdomain.SeverityMedium
	}
}

func (u *Usecases) emitAlert(ctx context.Context, item incidentdomain.Incident) {
	if u.alerts == nil {
		return
	}
	channel, route, ok := alertRouting(item.Severity)
	if !ok {
		return
	}
	if _, err := u.alerts.Create(ctx, alerts.CreateRequest{
		SourceKind:   alertdomain.SourceKindIncident,
		SourceID:     item.ID,
		ActionID:     item.SourceID,
		ResourceID:   item.ResourceID,
		ResourceType: item.ResourceType,
		Channel:      channel,
		Route:        route,
		Severity:     alertdomain.Severity(item.Severity),
		Status:       alertdomain.StatusPending,
		Summary:      item.Summary,
		Body:         alertBody(item),
		Details: map[string]any{
			"incident_id":     item.ID,
			"trigger":         string(item.Trigger),
			"resource_id":     item.ResourceID,
			"resource_type":   item.ResourceType,
			"risk_level":      string(item.RiskLevel),
			"incident_status": string(item.Status),
		},
	}); err != nil {
		log.Printf("control-workers alert sink failed: incident_id=%s severity=%s err=%v", item.ID, item.Severity, err)
	}
}

func alertRouting(severity incidentdomain.Severity) (alertdomain.Channel, string, bool) {
	switch severity {
	case incidentdomain.SeverityCritical:
		return alertdomain.ChannelPagerDuty, "ops-p1", true
	case incidentdomain.SeverityHigh:
		return alertdomain.ChannelSlack, "ops-p2", true
	default:
		return "", "", false
	}
}

func alertBody(item incidentdomain.Incident) string {
	if strings.TrimSpace(item.Reason) != "" {
		return item.Summary + ": " + strings.TrimSpace(item.Reason)
	}
	return item.Summary
}

func validateSourceKind(value incidentdomain.SourceKind) error {
	switch value {
	case incidentdomain.SourceKindAction:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported source_kind")
	}
}

func validateTrigger(value incidentdomain.Trigger) error {
	switch value {
	case incidentdomain.TriggerBlockedAction, incidentdomain.TriggerApprovalRejected, incidentdomain.TriggerApprovalExpired, incidentdomain.TriggerExecutionFailed:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported trigger")
	}
}

func validateSeverity(value incidentdomain.Severity) error {
	switch value {
	case incidentdomain.SeverityLow, incidentdomain.SeverityMedium, incidentdomain.SeverityHigh, incidentdomain.SeverityCritical:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported severity")
	}
}

func validateStatus(value incidentdomain.Status) error {
	switch value {
	case incidentdomain.StatusOpen, incidentdomain.StatusAcknowledged, incidentdomain.StatusResolved:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported status")
	}
}

func validateRiskLevel(value incidentdomain.RiskLevel) error {
	switch value {
	case incidentdomain.RiskLevelLow, incidentdomain.RiskLevelMedium, incidentdomain.RiskLevelHigh, incidentdomain.RiskLevelCritical:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported risk_level")
	}
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "incident not found")
	}
	if errors.Is(err, ErrArchived) {
		return newHTTPError(http.StatusConflict, "ARCHIVED", "incident is archived")
	}
	if errors.Is(err, ErrAlreadyArchived) {
		return newHTTPError(http.StatusConflict, "ALREADY_ARCHIVED", "incident already archived")
	}
	if errors.Is(err, ErrNotArchived) {
		return newHTTPError(http.StatusConflict, "NOT_ARCHIVED", "incident is not archived")
	}
	return err
}
