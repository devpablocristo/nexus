package contracts

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-saas/cmd/config"
	actionsvc "nexus-saas/internal/actions"
	"nexus-saas/internal/admin"
	admindomain "nexus-saas/internal/admin/usecases/domain"
	eventsvc "nexus-saas/internal/events"
	"nexus-saas/internal/usagemetering"
)

type Handler struct {
	cfg           config.ServiceConfig
	admin         entitlementsStore
	metering      usageSink
	events        *eventsvc.Usecases
	actions       *actionsvc.Usecases
	notifications notificationPort
}

type entitlementsStore interface {
	GetTenantSettings(ctx context.Context, orgID uuid.UUID) (admindomain.TenantSettings, bool, error)
}

type usageSink interface {
	IncrementEvent(ctx context.Context, eventID string, orgID uuid.UUID, counter string) error
	GetCounter(ctx context.Context, orgID uuid.UUID, counter string, period time.Time) (int64, error)
}

type notificationPort interface {
	Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error
}

func NewHandler(
	cfg config.ServiceConfig,
	adminRepo *admin.Repository,
	meteringRepo *usagemetering.Repository,
	eventsUC *eventsvc.Usecases,
	actionsUC *actionsvc.Usecases,
	notifications notificationPort,
) *Handler {
	return &Handler{
		cfg:           cfg,
		admin:         adminRepo,
		metering:      meteringRepo,
		events:        eventsUC,
		actions:       actionsUC,
		notifications: notifications,
	}
}

func (h *Handler) RegisterInternal(r *gin.Engine) {
	g := r.Group("/internal")
	g.Use(h.requireInternalKey)
	g.POST("/usage/events", h.ingestUsageEvent)
	g.POST("/events", h.ingestOperationalEvent)
	g.GET("/entitlements/:org_id", h.getEntitlements)
	g.GET("/runtime-overrides/:org_id/:tool_name", h.getRuntimeOverrides)
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

func usagePeriodUTC(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}
