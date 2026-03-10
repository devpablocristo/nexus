package dto

import "time"

type SessionItem struct {
	ID           string         `json:"id"`
	SessionID    string         `json:"session_id"`
	Actor        *string        `json:"actor,omitempty"`
	TotalCalls   int            `json:"total_calls"`
	TotalWrites  int            `json:"total_writes"`
	TotalDenials int            `json:"total_denials"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	LastCallAt   time.Time      `json:"last_call_at"`
}
