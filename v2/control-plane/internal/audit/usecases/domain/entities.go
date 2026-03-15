package domain

import (
	"time"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
)

// AuditRecord is the immutable record stored by Nexus audit.
type AuditRecord struct {
	ID            string
	EventType     string
	SourceService string
	ActionID      string
	ResourceID    string
	ResourceType  string
	Actor         *sharedaudit.Actor
	Summary       string
	Data          map[string]any
	OccurredAt    time.Time
	CreatedAt     time.Time
}
