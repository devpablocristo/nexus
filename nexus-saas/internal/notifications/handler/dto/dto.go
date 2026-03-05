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
	NotificationType string `json:"notification_type"`
	Enabled          bool   `json:"enabled"`
}

type UpdatePreferencesRequest struct {
	Items []PreferenceUpdate `json:"items"`
}
