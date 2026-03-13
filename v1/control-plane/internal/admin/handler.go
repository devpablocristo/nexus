package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	admindto "control-plane/internal/admin/handler/dto"
	admindomain "control-plane/internal/admin/usecases/domain"
	"nexus/pkg/http/errors"
	"nexus/pkg/types"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/admin/bootstrap", h.bootstrap)
	rg.GET("/admin/tenant-settings", h.getTenantSettings)
	rg.PUT("/admin/tenant-settings", h.upsertTenantSettings)
	rg.GET("/admin/protected-resources", h.listProtectedResources)
	rg.GET("/admin/restore-evidence", h.listRestoreEvidence)
	rg.POST("/admin/restore-evidence", h.recordRestoreEvidence)
	rg.POST("/admin/protected-resources", h.createProtectedResource)
	rg.DELETE("/admin/protected-resources/:id", h.deleteProtectedResource)
	rg.PUT("/admin/tenants/:org_id/suspend", h.suspendTenant)
	rg.PUT("/admin/tenants/:org_id/reactivate", h.reactivateTenant)
	rg.DELETE("/admin/tenants/:org_id", h.deleteTenant)
	rg.GET("/admin/activity", h.listActivity)
}

func (h *Handler) bootstrap(c *gin.Context) {
	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	role := roleFromCtx(c)
	scopes := scopesFromCtx(c)
	resp, err := h.uc.GetBootstrap(c.Request.Context(), orgID, actor, role, scopes, authMethodFromCtx(c))
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, admindto.BootstrapResponse{
		OrgID:         resp.OrgID.String(),
		Actor:         resp.Actor,
		Role:          resp.Role,
		Scopes:        resp.Scopes,
		AuthMethod:    resp.AuthMethod,
		CanReadAdmin:  resp.CanReadAdmin,
		CanWriteAdmin: resp.CanWriteAdmin,
		TenantSetting: toTenantSettingsDTO(resp.Settings),
	})
}

func (h *Handler) getTenantSettings(c *gin.Context) {
	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	role := roleFromCtx(c)
	scopes := scopesFromCtx(c)
	settings, err := h.uc.GetTenantSettings(c.Request.Context(), orgID, actor, role, scopes)
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toTenantSettingsDTO(settings))
}

func (h *Handler) upsertTenantSettings(c *gin.Context) {
	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	role := roleFromCtx(c)
	scopes := scopesFromCtx(c)
	var req admindto.UpsertTenantSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(c, "invalid json")
		return
	}
	settings, err := h.uc.UpsertTenantSettings(c.Request.Context(), orgID, actor, role, scopes, UpsertTenantSettingsRequest{
		PlanCode:   req.PlanCode,
		HardLimits: req.HardLimits,
	})
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toTenantSettingsDTO(settings))
}

func (h *Handler) listActivity(c *gin.Context) {
	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	role := roleFromCtx(c)
	scopes := scopesFromCtx(c)
	limit := parseLimit(c.Query("limit"))
	items, err := h.uc.ListActivity(c.Request.Context(), orgID, actor, role, scopes, limit)
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	resp := admindto.AdminActivityResponse{Items: make([]admindto.AdminActivityItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, admindto.AdminActivityItem{
			ID:           item.ID.String(),
			Actor:        item.Actor,
			Action:       item.Action,
			ResourceType: item.ResourceType,
			ResourceID:   item.ResourceID,
			Payload:      item.Payload,
			CreatedAt:    formatTime(item.CreatedAt),
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) listProtectedResources(c *gin.Context) {
	orgID := mustOrgID(c)
	items, err := h.uc.ListProtectedResources(c.Request.Context(), orgID, actorFromCtx(c), roleFromCtx(c), scopesFromCtx(c))
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	resp := admindto.ProtectedResourcesResponse{Items: make([]admindto.ProtectedResourceItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toProtectedResourceDTO(item))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) createProtectedResource(c *gin.Context) {
	orgID := mustOrgID(c)
	var req admindto.CreateProtectedResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(c, "invalid json")
		return
	}
	item, err := h.uc.CreateProtectedResource(c.Request.Context(), orgID, actorFromCtx(c), roleFromCtx(c), scopesFromCtx(c), CreateProtectedResourceRequest{
		Name:         req.Name,
		ResourceType: req.ResourceType,
		MatchValue:   req.MatchValue,
		MatchMode:    req.MatchMode,
		Environment:  req.Environment,
		Reason:       req.Reason,
		Enabled:      req.Enabled,
	})
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProtectedResourceDTO(item))
}

func (h *Handler) listRestoreEvidence(c *gin.Context) {
	orgID := mustOrgID(c)
	items, err := h.uc.ListRestoreEvidence(
		c.Request.Context(),
		orgID,
		actorFromCtx(c),
		roleFromCtx(c),
		scopesFromCtx(c),
		c.Query("environment"),
		parseLimit(c.Query("limit")),
	)
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	resp := admindto.RestoreEvidenceResponse{Items: make([]admindto.RestoreEvidenceItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toRestoreEvidenceDTO(item))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) recordRestoreEvidence(c *gin.Context) {
	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	role := roleFromCtx(c)
	scopes := scopesFromCtx(c)
	if _, canWrite := adminCapabilities(role, scopes); !canWrite {
		errors.WriteFrom(c, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console write permission required"))
		return
	}
	var req admindto.RecordRestoreEvidenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequest(c, "invalid json")
		return
	}
	startedAt, err := parseOptionalRFC3339(req.StartedAt)
	if err != nil {
		errors.BadRequest(c, "invalid started_at")
		return
	}
	completedAt, err := parseOptionalRFC3339(req.CompletedAt)
	if err != nil {
		errors.BadRequest(c, "invalid completed_at")
		return
	}
	item, err := h.uc.RecordRestoreEvidence(c.Request.Context(), orgID, actor, RecordRestoreEvidenceRequest{
		Environment:    req.Environment,
		System:         req.System,
		Status:         req.Status,
		SnapshotID:     req.SnapshotID,
		RestoreTarget:  req.RestoreTarget,
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		Source:         req.Source,
		ArtifactSHA256: req.ArtifactSHA256,
		Summary:        req.Summary,
	})
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, toRestoreEvidenceDTO(item))
}

func (h *Handler) deleteProtectedResource(c *gin.Context) {
	orgID := mustOrgID(c)
	resourceID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		errors.BadRequest(c, "invalid protected resource id")
		return
	}
	if err := h.uc.DeleteProtectedResource(c.Request.Context(), orgID, actorFromCtx(c), roleFromCtx(c), scopesFromCtx(c), resourceID); err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func toTenantSettingsDTO(s admindomain.TenantSettings) admindto.TenantSettings {
	return admindto.TenantSettings{
		PlanCode:   s.PlanCode,
		Status:     s.Status,
		DeletedAt:  formatTimePtr(s.DeletedAt),
		HardLimits: s.HardLimits,
		UpdatedBy:  s.UpdatedBy,
		UpdatedAt:  formatTime(s.UpdatedAt),
		CreatedAt:  formatTime(s.CreatedAt),
	}
}

func toProtectedResourceDTO(item admindomain.ProtectedResource) admindto.ProtectedResourceItem {
	return admindto.ProtectedResourceItem{
		ID:           item.ID.String(),
		Name:         item.Name,
		ResourceType: item.ResourceType,
		MatchValue:   item.MatchValue,
		MatchMode:    item.MatchMode,
		Environment:  item.Environment,
		Reason:       item.Reason,
		Enabled:      item.Enabled,
		CreatedBy:    item.CreatedBy,
		UpdatedBy:    item.UpdatedBy,
		CreatedAt:    formatTime(item.CreatedAt),
		UpdatedAt:    formatTime(item.UpdatedAt),
	}
}

func toRestoreEvidenceDTO(item admindomain.RestoreEvidence) admindto.RestoreEvidenceItem {
	return admindto.RestoreEvidenceItem{
		ID:             item.ID.String(),
		Environment:    item.Environment,
		System:         item.System,
		Status:         item.Status,
		SnapshotID:     item.SnapshotID,
		RestoreTarget:  item.RestoreTarget,
		StartedAt:      formatTimePtr(item.StartedAt),
		CompletedAt:    formatTimePtr(item.CompletedAt),
		Source:         item.Source,
		ArtifactSHA256: item.ArtifactSHA256,
		Summary:        item.Summary,
		CreatedAt:      formatTime(item.CreatedAt),
	}
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

func (h *Handler) suspendTenant(c *gin.Context) {
	actorOrgID := mustOrgID(c)
	targetOrgID, err := parseOrgParam(c)
	if err != nil {
		errors.BadRequest(c, "invalid org_id")
		return
	}
	settings, err := h.uc.SuspendTenant(c.Request.Context(), actorOrgID, targetOrgID, actorFromCtx(c), roleFromCtx(c), scopesFromCtx(c))
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toTenantSettingsDTO(settings))
}

func (h *Handler) reactivateTenant(c *gin.Context) {
	actorOrgID := mustOrgID(c)
	targetOrgID, err := parseOrgParam(c)
	if err != nil {
		errors.BadRequest(c, "invalid org_id")
		return
	}
	settings, err := h.uc.ReactivateTenant(c.Request.Context(), actorOrgID, targetOrgID, actorFromCtx(c), roleFromCtx(c), scopesFromCtx(c))
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toTenantSettingsDTO(settings))
}

func (h *Handler) deleteTenant(c *gin.Context) {
	actorOrgID := mustOrgID(c)
	targetOrgID, err := parseOrgParam(c)
	if err != nil {
		errors.BadRequest(c, "invalid org_id")
		return
	}
	settings, err := h.uc.DeleteTenant(c.Request.Context(), actorOrgID, targetOrgID, actorFromCtx(c), roleFromCtx(c), scopesFromCtx(c))
	if err != nil {
		errors.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toTenantSettingsDTO(settings))
}

func formatTime(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}

func formatTimePtr(v *time.Time) string {
	if v == nil {
		return ""
	}
	return formatTime(*v)
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, ok := c.Get(string(types.CtxKeyOrgID))
	if !ok {
		return uuid.Nil
	}
	id, _ := v.(uuid.UUID)
	return id
}

func actorFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyActor)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func roleFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyRole)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func scopesFromCtx(c *gin.Context) []string {
	if v, ok := c.Get(string(types.CtxKeyScopes)); ok {
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}

func authMethodFromCtx(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyAuthMethod)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func parseLimit(raw string) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 20
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 20
	}
	if n > 200 {
		return 200
	}
	return n
}

func parseOrgParam(c *gin.Context) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(c.Param("org_id")))
}
