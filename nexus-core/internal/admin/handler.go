package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	admindto "nexus-core/internal/admin/handler/dto"
	admindomain "nexus-core/internal/admin/usecases/domain"
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

func toTenantSettingsDTO(s admindomain.TenantSettings) admindto.TenantSettings {
	return admindto.TenantSettings{
		PlanCode:   s.PlanCode,
		HardLimits: s.HardLimits,
		UpdatedBy:  s.UpdatedBy,
		UpdatedAt:  formatTime(s.UpdatedAt),
		CreatedAt:  formatTime(s.CreatedAt),
	}
}

func formatTime(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
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
