package registry

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domain "github.com/devpablocristo/nexus/v3/companion/internal/connectors/usecases/domain"
)

// MockConnector conector de prueba que loguea sin ejecutar.
type MockConnector struct{}

// NewMockConnector crea un conector mock.
func NewMockConnector() *MockConnector {
	return &MockConnector{}
}

func (m *MockConnector) ID() string   { return "mock" }
func (m *MockConnector) Kind() string { return "mock" }

func (m *MockConnector) Capabilities() []domain.Capability {
	return []domain.Capability{
		{Operation: "mock.echo", SideEffect: false, ReadOnly: true},
		{Operation: "mock.write", SideEffect: true, ReadOnly: false},
	}
}

func (m *MockConnector) Validate(spec domain.ExecutionSpec) error {
	return nil
}

func (m *MockConnector) Execute(ctx context.Context, spec domain.ExecutionSpec) (domain.ExecutionResult, error) {
	slog.Info("mock connector execute",
		"operation", spec.Operation,
		"payload", string(spec.Payload),
		"idempotency_key", spec.IdempotencyKey,
	)
	resultJSON, _ := json.Marshal(map[string]string{
		"mock":    "true",
		"message": "operation logged successfully",
	})
	return domain.ExecutionResult{
		ID:          uuid.New(),
		ConnectorID: spec.ConnectorID,
		Operation:   spec.Operation,
		Status:      domain.ExecSuccess,
		ExternalRef: "mock-" + uuid.New().String()[:8],
		Payload:     spec.Payload,
		ResultJSON:  json.RawMessage(resultJSON),
		DurationMS:  1,
		TaskID:      spec.TaskID,
		ReviewRequestID: spec.ReviewRequestID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}
