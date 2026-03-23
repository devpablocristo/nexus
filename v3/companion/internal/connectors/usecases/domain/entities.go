package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Connector configuración de un conector a sistema externo.
type Connector struct {
	ID        uuid.UUID
	Name      string
	Kind      string // pymes, whatsapp, slack, email, calendar, mock
	Enabled   bool
	ConfigJSON json.RawMessage // credenciales/config (sin secretos en claro)
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Capability capacidad que ofrece un conector.
type Capability struct {
	Operation  string `json:"operation"`  // send_whatsapp, create_purchase, etc.
	SideEffect bool   `json:"side_effect"` // true = requiere gating por Review
	ReadOnly   bool   `json:"read_only"`
}

// ExecutionSpec especificación de una ejecución en un conector.
type ExecutionSpec struct {
	ConnectorID    uuid.UUID
	Operation      string
	Payload        json.RawMessage
	IdempotencyKey string
	TaskID         *uuid.UUID
	ReviewRequestID *uuid.UUID
}

// ExecutionResult resultado de una ejecución.
type ExecutionResult struct {
	ID            uuid.UUID
	ConnectorID   uuid.UUID
	Operation     string
	Status        string // success, failure, partial
	ExternalRef   string // referencia en el sistema externo
	Payload       json.RawMessage
	ResultJSON    json.RawMessage
	ErrorMessage  string
	Retryable     bool
	DurationMS    int64
	TaskID        *uuid.UUID
	ReviewRequestID *uuid.UUID
	CreatedAt     time.Time
}

// ExecutionStatus valores de estado de ejecución.
const (
	ExecSuccess = "success"
	ExecFailure = "failure"
	ExecPartial = "partial"
)
