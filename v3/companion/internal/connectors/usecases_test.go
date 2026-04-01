package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/nexus/v3/companion/internal/connectors/registry"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/connectors/usecases/domain"
)

type fakeConnectorRepo struct {
	connectors map[uuid.UUID]domain.Connector
	executions []domain.ExecutionResult
}

func (f *fakeConnectorRepo) SaveConnector(ctx context.Context, c domain.Connector) (domain.Connector, error) {
	if f.connectors == nil {
		f.connectors = make(map[uuid.UUID]domain.Connector)
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	f.connectors[c.ID] = c
	return c, nil
}

func (f *fakeConnectorRepo) GetConnector(ctx context.Context, id uuid.UUID) (domain.Connector, error) {
	c, ok := f.connectors[id]
	if !ok {
		return domain.Connector{}, ErrNotFound
	}
	return c, nil
}

func (f *fakeConnectorRepo) ListConnectors(ctx context.Context) ([]domain.Connector, error) {
	var out []domain.Connector
	for _, c := range f.connectors {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeConnectorRepo) UpdateConnector(ctx context.Context, c domain.Connector) (domain.Connector, error) {
	f.connectors[c.ID] = c
	return c, nil
}

func (f *fakeConnectorRepo) DeleteConnector(ctx context.Context, id uuid.UUID) error {
	delete(f.connectors, id)
	return nil
}

func (f *fakeConnectorRepo) SaveExecution(ctx context.Context, r domain.ExecutionResult) error {
	f.executions = append(f.executions, r)
	return nil
}

func (f *fakeConnectorRepo) ListExecutions(ctx context.Context, connectorID uuid.UUID, limit int) ([]domain.ExecutionResult, error) {
	var out []domain.ExecutionResult
	for _, execution := range f.executions {
		if execution.ConnectorID == connectorID {
			out = append(out, execution)
		}
	}
	return out, nil
}

type stubChecker struct {
	approved bool
	err      error
}

func (s *stubChecker) IsApproved(ctx context.Context, reviewRequestID uuid.UUID) (bool, error) {
	return s.approved, s.err
}

func TestUsecases_Execute_resolvesConnectorByKind(t *testing.T) {
	t.Parallel()
	repo := &fakeConnectorRepo{connectors: make(map[uuid.UUID]domain.Connector)}
	connectorID := uuid.New()
	repo.connectors[connectorID] = domain.Connector{
		ID:      connectorID,
		Name:    "Mock Connector",
		Kind:    "mock",
		Enabled: true,
	}
	reg := registry.NewRegistry()
	reg.Register(registry.NewMockConnector())
	uc := NewUsecases(repo, reg, &stubChecker{approved: true})
	reviewRequestID := uuid.New()

	result, err := uc.Execute(context.Background(), domain.ExecutionSpec{
		ConnectorID:     connectorID,
		Operation:       "mock.write",
		Payload:         json.RawMessage(`{"message":"hello"}`),
		ReviewRequestID: &reviewRequestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConnectorID != connectorID {
		t.Fatalf("unexpected connector id %s", result.ConnectorID)
	}
	if len(repo.executions) != 1 {
		t.Fatalf("expected persisted execution, got %d", len(repo.executions))
	}
}

func TestUsecases_Execute_disabledConnector(t *testing.T) {
	t.Parallel()
	repo := &fakeConnectorRepo{connectors: make(map[uuid.UUID]domain.Connector)}
	connectorID := uuid.New()
	repo.connectors[connectorID] = domain.Connector{
		ID:      connectorID,
		Name:    "Mock Connector",
		Kind:    "mock",
		Enabled: false,
	}
	reg := registry.NewRegistry()
	reg.Register(registry.NewMockConnector())
	uc := NewUsecases(repo, reg, &stubChecker{approved: true})

	_, err := uc.Execute(context.Background(), domain.ExecutionSpec{
		ConnectorID: connectorID,
		Operation:   "mock.echo",
		Payload:     json.RawMessage(`{}`),
	})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}
