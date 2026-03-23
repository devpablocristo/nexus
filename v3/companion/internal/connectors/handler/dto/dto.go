package dto

import (
	"encoding/json"
	"time"

	domain "github.com/devpablocristo/nexus/v3/companion/internal/connectors/usecases/domain"
)

// ConnectorResponse respuesta de un conector.
type ConnectorResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Enabled   bool            `json:"enabled"`
	Config    json.RawMessage `json:"config"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

// ConnectorListResponse lista de conectores.
type ConnectorListResponse struct {
	Connectors []ConnectorResponse `json:"connectors"`
}

// ExecuteRequest petición para ejecutar una operación.
type ExecuteRequest struct {
	ConnectorID     string          `json:"connector_id"`
	Operation       string          `json:"operation"`
	Payload         json.RawMessage `json:"payload"`
	IdempotencyKey  string          `json:"idempotency_key,omitempty"`
	TaskID          string          `json:"task_id,omitempty"`
	ReviewRequestID string          `json:"review_request_id,omitempty"`
}

// ExecutionResponse resultado de una ejecución.
type ExecutionResponse struct {
	ID              string          `json:"id"`
	ConnectorID     string          `json:"connector_id"`
	Operation       string          `json:"operation"`
	Status          string          `json:"status"`
	ExternalRef     string          `json:"external_ref"`
	Result          json.RawMessage `json:"result"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	DurationMS      int64           `json:"duration_ms"`
	CreatedAt       string          `json:"created_at"`
}

// ExecutionListResponse lista de ejecuciones.
type ExecutionListResponse struct {
	Executions []ExecutionResponse `json:"executions"`
}

// CapabilityResponse capacidad de un conector.
type CapabilityResponse struct {
	ConnectorID  string               `json:"connector_id"`
	Kind         string               `json:"kind"`
	Capabilities []domain.Capability  `json:"capabilities"`
}

// CapabilitiesListResponse lista de capacidades.
type CapabilitiesListResponse struct {
	Connectors []CapabilityResponse `json:"connectors"`
}

// SaveConnectorRequest petición para guardar un conector.
type SaveConnectorRequest struct {
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config,omitempty"`
}

// ConnectorToResponse convierte entidad a DTO.
func ConnectorToResponse(c domain.Connector) ConnectorResponse {
	return ConnectorResponse{
		ID:        c.ID.String(),
		Name:      c.Name,
		Kind:      c.Kind,
		Enabled:   c.Enabled,
		Config:    c.ConfigJSON,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// ExecutionToResponse convierte resultado a DTO.
func ExecutionToResponse(e domain.ExecutionResult) ExecutionResponse {
	return ExecutionResponse{
		ID:           e.ID.String(),
		ConnectorID:  e.ConnectorID.String(),
		Operation:    e.Operation,
		Status:       e.Status,
		ExternalRef:  e.ExternalRef,
		Result:       e.ResultJSON,
		ErrorMessage: e.ErrorMessage,
		DurationMS:   e.DurationMS,
		CreatedAt:    e.CreatedAt.UTC().Format(time.RFC3339),
	}
}
