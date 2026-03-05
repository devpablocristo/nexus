package notifications

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	notificationdto "nexus-saas/internal/notifications/handler/dto"
	notificationdomain "nexus-saas/internal/notifications/usecases/domain"
	httperr "nexus/pkg/http/errors"
	"nexus/pkg/types"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/notifications/preferences", h.getPreferences)
	rg.PUT("/notifications/preferences", h.updatePreferences)
}

func (h *Handler) getPreferences(c *gin.Context) {
	actor := strings.TrimSpace(actorFromCtx(c))
	if actor == "" {
		httperr.Write(c, http.StatusUnauthorized, types.ErrCodeUnauthorized, "missing user actor")
		return
	}
	items, err := h.uc.GetPreferencesByExternalID(c.Request.Context(), actor)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := notificationdto.PreferencesResponse{Items: make([]notificationdto.NotificationPreferenceItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, notificationdto.NotificationPreferenceItem{
			NotificationType: string(item.NotificationType),
			Channel:          notificationdomain.ChannelEmail,
			Enabled:          item.Enabled,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) updatePreferences(c *gin.Context) {
	actor := strings.TrimSpace(actorFromCtx(c))
	if actor == "" {
		httperr.Write(c, http.StatusUnauthorized, types.ErrCodeUnauthorized, "missing user actor")
		return
	}

	var req notificationdto.UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	if len(req.Items) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	updates := make(map[string]bool, len(req.Items))
	for _, item := range req.Items {
		notificationType := strings.TrimSpace(item.NotificationType)
		if notificationType == "" {
			httperr.BadRequest(c, "notification_type is required")
			return
		}
		updates[notificationType] = item.Enabled
	}
	if err := h.uc.UpdatePreferencesByExternalID(c.Request.Context(), actor, updates); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func actorFromCtx(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyActor)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
