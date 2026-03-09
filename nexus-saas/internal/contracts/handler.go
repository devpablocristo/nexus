package contracts

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-saas/cmd/config"
	actionsvc "nexus-saas/internal/actions"
	actiondomain "nexus-saas/internal/actions/usecases/domain"
	"nexus-saas/internal/admin"
	admindomain "nexus-saas/internal/admin/usecases/domain"
	"nexus-saas/internal/billing"
	billingdomain "nexus-saas/internal/billing/usecases/domain"
	eventsvc "nexus-saas/internal/events"
	eventdomain "nexus-saas/internal/events/usecases/domain"
	"nexus-saas/internal/incidents"
	incidentdomain "nexus-saas/internal/incidents/usecases/domain"
	"nexus-saas/internal/notifications"
	"nexus-saas/internal/policyproposal"
	proposaldomain "nexus-saas/internal/policyproposal/usecases/domain"
	"nexus-saas/internal/usagemetering"
)

type Handler struct {
	cfg           config.ServiceConfig
	admin         entitlementsStore
	billing       billingReader
	metering      usageSink
	events        eventsReader
	actions       actionsReader
	incidents     incidentsReader
	proposals     proposalsReader
	notifications notificationPort
}

type entitlementsStore interface {
	GetTenantSettings(ctx context.Context, orgID uuid.UUID) (admindomain.TenantSettings, bool, error)
	ListProtectedResources(ctx context.Context, orgID uuid.UUID) ([]admindomain.ProtectedResource, error)
	ListRestoreEvidence(ctx context.Context, orgID uuid.UUID, environment string, limit int) ([]admindomain.RestoreEvidence, error)
	CreateRestoreEvidence(ctx context.Context, evidence admindomain.RestoreEvidence) (admindomain.RestoreEvidence, error)
}

type usageSink interface {
	IncrementEvent(ctx context.Context, eventID string, orgID uuid.UUID, counter string) error
	GetCounter(ctx context.Context, orgID uuid.UUID, counter string, period time.Time) (int64, error)
}

type billingReader interface {
	GetTenantBilling(ctx context.Context, orgID uuid.UUID) (billingdomain.TenantBilling, bool, error)
	GetUsageSummary(ctx context.Context, orgID uuid.UUID, period time.Time) (billingdomain.UsageSummary, error)
}

type eventsReader interface {
	Append(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error)
	ListRecent(ctx context.Context, orgID uuid.UUID, limit int) ([]eventdomain.Event, error)
}

type actionsReader interface {
	List(ctx context.Context, orgID uuid.UUID, q actionsvc.ListQuery) ([]actiondomain.Action, error)
	ResolveRuntimeOverrides(ctx context.Context, orgID uuid.UUID, toolName string) (actiondomain.RuntimeOverrides, error)
}

type incidentsReader interface {
	List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]incidentdomain.Incident, error)
}

type proposalsReader interface {
	List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]proposaldomain.Proposal, error)
}

type notificationPort interface {
	Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error
}

func NewHandler(
	cfg config.ServiceConfig,
	adminRepo *admin.Repository,
	billingRepo *billing.Repository,
	meteringRepo *usagemetering.Repository,
	eventsUC *eventsvc.Usecases,
	actionsUC *actionsvc.Usecases,
	incidentsUC *incidents.Usecases,
	proposalsUC *policyproposal.Usecases,
	notificationsUC *notifications.Usecases,
) *Handler {
	return &Handler{
		cfg:           cfg,
		admin:         adminRepo,
		billing:       billingRepo,
		metering:      meteringRepo,
		events:        eventsUC,
		actions:       actionsUC,
		incidents:     incidentsUC,
		proposals:     proposalsUC,
		notifications: notificationsUC,
	}
}

func (h *Handler) RegisterInternal(r *gin.Engine) {
	g := r.Group("/internal")
	g.Use(h.requireInternalKey)
	g.POST("/usage/events", h.ingestUsageEvent)
	g.POST("/events", h.ingestOperationalEvent)
	g.GET("/entitlements/:org_id", h.getEntitlements)
	g.GET("/assistant/context/:org_id", h.getAssistantContext)
	g.GET("/runtime-overrides/:org_id/:tool_name", h.getRuntimeOverrides)
	g.GET("/protected-resources/:org_id", h.getProtectedResources)
	g.GET("/restore-evidence/:org_id", h.getRestoreEvidence)
	g.POST("/restore-evidence", h.recordRestoreEvidence)
}

func (h *Handler) requireInternalKey(c *gin.Context) {
	expected := strings.TrimSpace(h.cfg.SaaSInternalKey)
	if expected == "" {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "CONFIG_ERROR", "message": "NEXUS_SAAS_INTERNAL_KEY not configured"},
		})
		return
	}
	got := strings.TrimSpace(c.GetHeader("X-NEXUS-SAAS-KEY"))
	if got == "" || got != expected {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "UNAUTHORIZED", "message": "invalid internal key"},
		})
		return
	}
	c.Next()
}

type usageEventRequest struct {
	EventID string `json:"event_id"`
	OrgID   string `json:"org_id" binding:"required"`
	Counter string `json:"counter" binding:"required"`
}

type operationalEventRequest struct {
	OrgID     string         `json:"org_id" binding:"required"`
	EventType string         `json:"event_type" binding:"required"`
	Payload   map[string]any `json:"payload"`
}

type restoreEvidenceRequest struct {
	OrgID          string         `json:"org_id" binding:"required"`
	Environment    string         `json:"environment"`
	System         string         `json:"system"`
	Status         string         `json:"status"`
	SnapshotID     string         `json:"snapshot_id"`
	RestoreTarget  string         `json:"restore_target"`
	StartedAt      string         `json:"started_at"`
	CompletedAt    string         `json:"completed_at"`
	Source         string         `json:"source"`
	ArtifactSHA256 string         `json:"artifact_sha256"`
	Summary        map[string]any `json:"summary"`
}

func (h *Handler) ingestUsageEvent(c *gin.Context) {
	var req usageEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid json"}})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(req.OrgID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}
	counter := strings.TrimSpace(req.Counter)
	if counter == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "counter required"}})
		return
	}
	if err := h.metering.IncrementEvent(c.Request.Context(), strings.TrimSpace(req.EventID), orgID, counter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to ingest usage event"}})
		return
	}
	h.checkUsageThresholds(c.Request.Context(), orgID, counter)
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

func (h *Handler) ingestOperationalEvent(c *gin.Context) {
	if h.events == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "UNAVAILABLE", "message": "events service unavailable"}})
		return
	}
	var req operationalEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid json"}})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(req.OrgID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "event_type required"}})
		return
	}
	if _, err := h.events.Append(c.Request.Context(), orgID, eventType, req.Payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to append event"}})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

func (h *Handler) getEntitlements(c *gin.Context) {
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}
	settings, ok, err := h.admin.GetTenantSettings(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to read entitlements"}})
		return
	}
	if !ok {
		settings.PlanCode = "starter"
		settings.Status = admindomain.TenantStatusActive
		settings.HardLimits = map[string]any{
			"tools_max":            20,
			"run_rpm":              300,
			"audit_retention_days": 30,
		}
	}
	if strings.TrimSpace(settings.Status) == "" {
		settings.Status = admindomain.TenantStatusActive
	}
	if settings.HardLimits == nil {
		settings.HardLimits = map[string]any{}
	}
	c.JSON(http.StatusOK, gin.H{
		"org_id":      orgID.String(),
		"plan_code":   settings.PlanCode,
		"status":      settings.Status,
		"hard_limits": settings.HardLimits,
	})
}

func (h *Handler) getRuntimeOverrides(c *gin.Context) {
	if h.actions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "UNAVAILABLE", "message": "actions service unavailable"}})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}
	toolName := strings.TrimSpace(c.Param("tool_name"))
	if toolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "tool_name required"}})
		return
	}
	overrides, err := h.actions.ResolveRuntimeOverrides(c.Request.Context(), orgID, toolName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to resolve runtime overrides"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"deny":                overrides.Deny,
		"deny_reason":         overrides.DenyReason,
		"tenant_rpm_override": overrides.TenantRPMOverride,
		"tool_rpm_override":   overrides.ToolRPMOverride,
	})
}

func (h *Handler) getProtectedResources(c *gin.Context) {
	if h.admin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "UNAVAILABLE", "message": "protected resources unavailable"}})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}
	items, err := h.admin.ListProtectedResources(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to read protected resources"}})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		out = append(out, gin.H{
			"id":            item.ID.String(),
			"name":          item.Name,
			"resource_type": item.ResourceType,
			"match_value":   item.MatchValue,
			"match_mode":    item.MatchMode,
			"environment":   item.Environment,
			"reason":        item.Reason,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"org_id": orgID.String(),
		"items":  out,
	})
}

func (h *Handler) getRestoreEvidence(c *gin.Context) {
	if h.admin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "UNAVAILABLE", "message": "restore evidence unavailable"}})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}
	environment := strings.TrimSpace(strings.ToLower(c.Query("environment")))
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := h.admin.ListRestoreEvidence(c.Request.Context(), orgID, environment, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to read restore evidence"}})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"id":              item.ID.String(),
			"environment":     item.Environment,
			"system":          item.System,
			"status":          item.Status,
			"snapshot_id":     item.SnapshotID,
			"restore_target":  item.RestoreTarget,
			"started_at":      formatTimePtr(item.StartedAt),
			"completed_at":    formatTimePtr(item.CompletedAt),
			"source":          item.Source,
			"artifact_sha256": item.ArtifactSHA256,
			"summary":         item.Summary,
			"created_at":      item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"org_id":      orgID.String(),
		"environment": environment,
		"items":       out,
	})
}

func (h *Handler) recordRestoreEvidence(c *gin.Context) {
	if h.admin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "UNAVAILABLE", "message": "restore evidence unavailable"}})
		return
	}
	var req restoreEvidenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid json"}})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(req.OrgID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}
	startedAt, err := parseOptionalRFC3339(req.StartedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid started_at"}})
		return
	}
	completedAt, err := parseOptionalRFC3339(req.CompletedAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid completed_at"}})
		return
	}
	item, err := h.admin.CreateRestoreEvidence(c.Request.Context(), admindomain.RestoreEvidence{
		ID:             uuid.New(),
		OrgID:          orgID,
		Environment:    strings.TrimSpace(strings.ToLower(req.Environment)),
		System:         strings.TrimSpace(strings.ToLower(req.System)),
		Status:         strings.TrimSpace(strings.ToLower(req.Status)),
		SnapshotID:     strings.TrimSpace(req.SnapshotID),
		RestoreTarget:  strings.TrimSpace(req.RestoreTarget),
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		Source:         strings.TrimSpace(req.Source),
		ArtifactSHA256: strings.TrimSpace(req.ArtifactSHA256),
		Summary:        req.Summary,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to record restore evidence"}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":              item.ID.String(),
		"org_id":          item.OrgID.String(),
		"environment":     item.Environment,
		"system":          item.System,
		"status":          item.Status,
		"snapshot_id":     item.SnapshotID,
		"restore_target":  item.RestoreTarget,
		"started_at":      formatTimePtr(item.StartedAt),
		"completed_at":    formatTimePtr(item.CompletedAt),
		"source":          item.Source,
		"artifact_sha256": item.ArtifactSHA256,
		"summary":         item.Summary,
		"created_at":      item.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) getAssistantContext(c *gin.Context) {
	if h.admin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "UNAVAILABLE", "message": "assistant context unavailable"}})
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid org_id"}})
		return
	}

	settings, ok, err := h.admin.GetTenantSettings(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to read tenant settings"}})
		return
	}
	if !ok {
		settings = admindomain.TenantSettings{
			OrgID:    orgID,
			PlanCode: "starter",
			Status:   admindomain.TenantStatusActive,
			HardLimits: map[string]any{
				"tools_max":            20,
				"run_rpm":              300,
				"audit_retention_days": 30,
			},
		}
	}
	if strings.TrimSpace(settings.Status) == "" {
		settings.Status = admindomain.TenantStatusActive
	}
	if settings.HardLimits == nil {
		settings.HardLimits = map[string]any{}
	}

	billingStatus := ""
	usagePeriod := usagePeriodUTC(time.Now().UTC()).Format("2006-01")
	usage := gin.H{
		"api_calls":        int64(0),
		"events_ingested":  int64(0),
		"incidents_opened": int64(0),
		"actions_executed": int64(0),
	}
	if h.billing != nil {
		if tenantBilling, found, billingErr := h.billing.GetTenantBilling(c.Request.Context(), orgID); billingErr == nil && found {
			billingStatus = string(tenantBilling.BillingStatus)
			if settings.PlanCode == "" {
				settings.PlanCode = string(tenantBilling.PlanCode)
			}
			if len(settings.HardLimits) == 0 {
				settings.HardLimits = map[string]any{
					"tools_max":            tenantBilling.HardLimits.ToolsMax,
					"run_rpm":              tenantBilling.HardLimits.RunRPM,
					"audit_retention_days": tenantBilling.HardLimits.AuditRetentionDays,
				}
			}
		}
		if summary, usageErr := h.billing.GetUsageSummary(c.Request.Context(), orgID, usagePeriodUTC(time.Now().UTC())); usageErr == nil {
			if strings.TrimSpace(summary.Period) != "" {
				usagePeriod = summary.Period
			}
			usage = gin.H{
				"api_calls":        summary.Counters.APICalls,
				"events_ingested":  summary.Counters.EventsIngested,
				"incidents_opened": summary.Counters.IncidentsOpened,
				"actions_executed": summary.Counters.ActionsExecuted,
			}
		}
	}

	openItems := make([]gin.H, 0, 5)
	openCount := 0
	if h.incidents != nil {
		if items, listErr := h.incidents.List(c.Request.Context(), orgID, string(incidentdomain.StatusOpen), 100); listErr == nil {
			openCount = len(items)
			for idx, item := range items {
				if idx >= 5 {
					break
				}
				openItems = append(openItems, gin.H{
					"id":        item.ID.String(),
					"severity":  string(item.Severity),
					"status":    string(item.Status),
					"title":     item.Title,
					"summary":   item.Summary,
					"opened_at": item.OpenedAt.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	actionItems := make([]gin.H, 0, 5)
	activeCount := 0
	if h.actions != nil {
		if items, listErr := h.actions.List(c.Request.Context(), orgID, actionsvc.ListQuery{Status: string(actiondomain.StatusActive), Limit: 100}); listErr == nil {
			activeCount = len(items)
			for idx, item := range items {
				if idx >= 5 {
					break
				}
				scopeID := ""
				if item.ScopeID != nil {
					scopeID = *item.ScopeID
				}
				actionItems = append(actionItems, gin.H{
					"id":          item.ID.String(),
					"action_type": string(item.ActionType),
					"status":      string(item.Status),
					"scope_type":  string(item.ScopeType),
					"scope_id":    scopeID,
					"summary":     summarizeAction(item),
					"created_at":  item.CreatedAt.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	proposalItems := make([]gin.H, 0, 5)
	pendingCount := 0
	if h.proposals != nil {
		if items, listErr := h.proposals.List(c.Request.Context(), orgID, "", 100); listErr == nil {
			for _, item := range items {
				if !isPendingProposal(item.Status) {
					continue
				}
				pendingCount++
				if len(proposalItems) >= 5 {
					continue
				}
				proposalItems = append(proposalItems, gin.H{
					"id":         item.ID.String(),
					"status":     string(item.Status),
					"rationale":  item.Rationale,
					"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	eventItems := make([]gin.H, 0, 5)
	if h.events != nil {
		if items, listErr := h.events.ListRecent(c.Request.Context(), orgID, 5); listErr == nil {
			for _, item := range items {
				eventItems = append(eventItems, gin.H{
					"id":         item.ID,
					"event_type": item.EventType,
					"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
					"summary":    summarizeEvent(item),
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"org_id":       orgID.String(),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"tenant": gin.H{
			"plan_code":   settings.PlanCode,
			"status":      settings.Status,
			"hard_limits": settings.HardLimits,
		},
		"billing": gin.H{
			"billing_status": billingStatus,
			"usage_period":   usagePeriod,
			"usage":          usage,
		},
		"incidents": gin.H{
			"open_count": openCount,
			"items":      openItems,
		},
		"actions": gin.H{
			"active_count": activeCount,
			"items":        actionItems,
		},
		"proposals": gin.H{
			"pending_count": pendingCount,
			"items":         proposalItems,
		},
		"events": gin.H{
			"recent_count": len(eventItems),
			"items":        eventItems,
		},
	})
}

func (h *Handler) checkUsageThresholds(ctx context.Context, orgID uuid.UUID, metricName string) {
	if h.notifications == nil {
		return
	}
	settings, ok, err := h.admin.GetTenantSettings(ctx, orgID)
	if err != nil || !ok {
		return
	}

	limit := usageLimitForCounter(metricName, settings.HardLimits)
	if limit <= 0 {
		return
	}
	period := usagePeriodUTC(time.Now().UTC())
	current, err := h.metering.GetCounter(ctx, orgID, metricName, period)
	if err != nil || current <= 0 {
		return
	}
	pct := float64(current) / float64(limit) * 100

	notifType := ""
	threshold := ""
	switch {
	case pct >= 100:
		notifType = "usage_limit_reached"
		threshold = "100"
	case pct >= 95:
		notifType = "usage_warning_95"
		threshold = "95"
	case pct >= 80:
		notifType = "usage_warning_80"
		threshold = "80"
	default:
		return
	}

	periodKey := period.Format("2006-01")
	data := map[string]string{
		"metric":       metricName,
		"current":      fmt.Sprintf("%d", current),
		"limit":        fmt.Sprintf("%d", limit),
		"pct":          threshold,
		"action_url":   "/billing",
		"reference_id": fmt.Sprintf("usage:%s:%s:%s:%s", orgID.String(), metricName, threshold, periodKey),
		"dedup_bucket": periodKey,
	}
	go func(payload map[string]string) {
		_ = h.notifications.Notify(context.Background(), orgID, notifType, payload)
	}(data)
}

func usageLimitForCounter(counter string, hardLimits map[string]any) int64 {
	if hardLimits == nil {
		return 0
	}
	if v, ok := hardLimits[counter]; ok {
		return asInt64(v)
	}
	switch counter {
	case usagemetering.CounterAPICalls:
		return asInt64(hardLimits["run_rpm"])
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

func summarizeAction(item actiondomain.Action) string {
	scopeID := ""
	if item.ScopeID != nil && strings.TrimSpace(*item.ScopeID) != "" {
		scopeID = " " + strings.TrimSpace(*item.ScopeID)
	}
	return fmt.Sprintf("%s %s on %s%s", item.ActionType, item.Status, item.ScopeType, scopeID)
}

func summarizeEvent(item eventdomain.Event) string {
	candidateKeys := []string{"title", "summary", "incident_id", "action_type", "proposal_id", "status"}
	parts := []string{item.EventType}
	for _, key := range candidateKeys {
		value := strings.TrimSpace(stringFromMap(item.Payload, key))
		if value == "" {
			continue
		}
		parts = append(parts, value)
		if len(parts) >= 3 {
			break
		}
	}
	return strings.Join(parts, " | ")
}

func stringFromMap(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func isPendingProposal(status proposaldomain.Status) bool {
	switch status {
	case proposaldomain.StatusDraft, proposaldomain.StatusPending, proposaldomain.StatusShadow:
		return true
	default:
		return false
	}
}

func usagePeriodUTC(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func parseOptionalRFC3339(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func formatTimePtr(v *time.Time) string {
	if v == nil || v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
