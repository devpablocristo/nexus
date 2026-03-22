package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TaskStatus valores persistidos (CHECK en SQL).
const (
	TaskStatusNew                = "new"
	TaskStatusInvestigating      = "investigating"
	TaskStatusProposing          = "proposing"
	TaskStatusWaitingForInput    = "waiting_for_input"
	TaskStatusWaitingForApproval = "waiting_for_approval"
	TaskStatusExecuting          = "executing"
	TaskStatusVerifying          = "verifying"
	TaskStatusDone               = "done"
	TaskStatusFailed             = "failed"
	TaskStatusEscalated          = "escalated"
)

// Task entidad de dominio.
type Task struct {
	ID          uuid.UUID
	Title       string
	Goal        string
	Status      string
	Priority    string
	CreatedBy   string
	AssignedTo  string
	Channel     string
	Summary     string
	ContextJSON json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ClosedAt    *time.Time
}

// TaskMessage mensaje en el hilo de una tarea.
type TaskMessage struct {
	ID         uuid.UUID
	TaskID     uuid.UUID
	AuthorType string
	AuthorID   string
	Body       string
	Metadata   json.RawMessage
	CreatedAt  time.Time
}

// TaskAction acción sobre una tarea (p. ej. propose → Review).
type TaskAction struct {
	ID               uuid.UUID
	TaskID           uuid.UUID
	ActionType       string
	Payload          json.RawMessage
	ReviewRequestID  *uuid.UUID
	ErrorMessage     string
	CreatedAt        time.Time
}

// TaskArtifact adjunto mínimo.
type TaskArtifact struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	Kind      string
	URI       string
	Payload   json.RawMessage
	CreatedAt time.Time
}
