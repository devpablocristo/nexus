package notifications

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	notificationdto "control-plane/internal/notifications/handler/dto"
	notificationdomain "control-plane/internal/notifications/usecases/domain"
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
	rg.GET("/notifications", h.listNotifications)
	rg.GET("/notifications/unread-count", h.unreadCount)
	rg.PUT("/notifications/:id/read", h.markNotificationRead)
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

func orgIDFromCtx(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(string(types.CtxKeyOrgID)); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

func (h *Handler) listNotifications(c *gin.Context) {
	actor := strings.TrimSpace(actorFromCtx(c))
	if actor == "" {
		httperr.Write(c, http.StatusUnauthorized, types.ErrCodeUnauthorized, "missing user actor")
		return
	}
	orgID := orgIDFromCtx(c)
	limit := parseIntDefault(c.Query("limit"), 20, 1, 200)
	offset := parseIntDefault(c.Query("offset"), 0, 0, 100000)
	items, err := h.uc.ListInAppNotifications(c.Request.Context(), orgID, actor, limit, offset)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := notificationdto.InAppNotificationsResponse{
		Items: make([]notificationdto.InAppNotificationItem, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, notificationdto.InAppNotificationItem{
			ID:        item.ID.String(),
			OrgID:     item.OrgID.String(),
			ActorID:   item.ActorID,
			Type:      item.Type,
			Title:     item.Title,
			Body:      item.Body,
			ReadAt:    formatTimePtr(item.ReadAt),
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) unreadCount(c *gin.Context) {
	actor := strings.TrimSpace(actorFromCtx(c))
	if actor == "" {
		httperr.Write(c, http.StatusUnauthorized, types.ErrCodeUnauthorized, "missing user actor")
		return
	}
	orgID := orgIDFromCtx(c)
	count, err := h.uc.GetUnreadInAppCount(c.Request.Context(), orgID, actor)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, notificationdto.UnreadCountResponse{Count: count})
}

func (h *Handler) markNotificationRead(c *gin.Context) {
	actor := strings.TrimSpace(actorFromCtx(c))
	if actor == "" {
		httperr.Write(c, http.StatusUnauthorized, types.ErrCodeUnauthorized, "missing user actor")
		return
	}
	orgID := orgIDFromCtx(c)
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		httperr.BadRequest(c, "invalid notification id")
		return
	}
	if err := h.uc.MarkInAppRead(c.Request.Context(), orgID, actor, id); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseIntDefault(raw string, def, min, max int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		v = min
	}
	if max > 0 && v > max {
		v = max
	}
	return v
}

func formatTimePtr(v *time.Time) *string {
	if v == nil {
		return nil
	}
	s := v.UTC().Format(time.RFC3339)
	return &s
}
