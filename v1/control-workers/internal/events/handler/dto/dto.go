package dto

type EventItem struct {
	ID        int64          `json:"id"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt string         `json:"created_at"`
}

type ListEventsResponse struct {
	Items      []EventItem `json:"items"`
	NextCursor int64       `json:"next_cursor"`
}
