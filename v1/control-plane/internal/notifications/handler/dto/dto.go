package dto

type NotificationPreferenceItem struct {
	NotificationType string `json:"notification_type"`
	Channel          string `json:"channel"`
	Enabled          bool   `json:"enabled"`
}

type PreferencesResponse struct {
	Items []NotificationPreferenceItem `json:"items"`
}

type PreferenceUpdate struct {
	NotificationType string `json:"notification_type" binding:"required"`
	Enabled          bool   `json:"enabled"`
}

type UpdatePreferencesRequest struct {
	Items []PreferenceUpdate `json:"items" binding:"required"`
}

type InAppNotificationItem struct {
	ID        string  `json:"id"`
	OrgID     string  `json:"org_id"`
	ActorID   string  `json:"actor_id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	ReadAt    *string `json:"read_at,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type InAppNotificationsResponse struct {
	Items []InAppNotificationItem `json:"items"`
}

type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
