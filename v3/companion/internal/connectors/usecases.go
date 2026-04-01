package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/nexus/v3/companion/internal/connectors/registry"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/connectors/usecases/domain"
)

// Repository port de persistencia para conectores y resultados de ejecución.
type Repository interface {
	SaveConnector(ctx context.Context, c domain.Connector) (domain.Connector, error)
	GetConnector(ctx context.Context, id uuid.UUID) (domain.Connector, error)
	ListConnectors(ctx context.Context) ([]domain.Connector, error)
	UpdateConnector(ctx context.Context, c domain.Connector) (domain.Connector, error)
	DeleteConnector(ctx context.Context, id uuid.UUID) error

	SaveExecution(ctx context.Context, r domain.ExecutionResult) error
	ListExecutions(ctx context.Context, connectorID uuid.UUID, limit int) ([]domain.ExecutionResult, error)
}

// ReviewChecker verifica que una ejecución tiene aprobación de Review.
type ReviewChecker interface {
	IsApproved(ctx context.Context, reviewRequestID uuid.UUID) (bool, error)
}

// Usecases lógica de negocio de conectores.
type Usecases struct {
	repo     Repository
	registry *registry.Registry
	checker  ReviewChecker
}

// NewUsecases crea una nueva instancia de Usecases.
func NewUsecases(repo Repository, reg *registry.Registry, checker ReviewChecker) *Usecases {
	return &Usecases{
		repo:     repo,
		registry: reg,
		checker:  checker,
	}
}

// ListConnectors lista conectores registrados con su estado en DB.
func (uc *Usecases) ListConnectors(ctx context.Context) ([]domain.Connector, error) {
	conns, err := uc.repo.ListConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}
	return conns, nil
}

// GetConnector obtiene un conector por ID.
func (uc *Usecases) GetConnector(ctx context.Context, id uuid.UUID) (domain.Connector, error) {
	conn, err := uc.repo.GetConnector(ctx, id)
	if err != nil {
		return domain.Connector{}, fmt.Errorf("get connector: %w", err)
	}
	return conn, nil
}

// SaveConnector crea o actualiza un conector.
func (uc *Usecases) SaveConnector(ctx context.Context, c domain.Connector) (domain.Connector, error) {
	if c.ID == uuid.Nil {
		return uc.repo.SaveConnector(ctx, c)
	}
	return uc.repo.UpdateConnector(ctx, c)
}

// DeleteConnector elimina un conector.
func (uc *Usecases) DeleteConnector(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteConnector(ctx, id)
}

// Execute ejecuta una operación en un conector con gating obligatorio.
func (uc *Usecases) Execute(ctx context.Context, spec domain.ExecutionSpec) (domain.ExecutionResult, error) {
	config, err := uc.repo.GetConnector(ctx, spec.ConnectorID)
	if err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("get connector config: %w", err)
	}
	if !config.Enabled {
		return domain.ExecutionResult{}, ErrDisabled
	}

	// Obtener implementación del conector a partir del kind persistido.
	conn, ok := uc.registry.Get(config.Kind)
	if !ok {
		return domain.ExecutionResult{}, ErrNotFound
	}

	// Verificar que la operación tiene side effects
	hasSideEffect := false
	operationKnown := false
	for _, cap := range conn.Capabilities() {
		if cap.Operation != spec.Operation {
			continue
		}
		operationKnown = true
		hasSideEffect = cap.SideEffect
		break
	}
	if !operationKnown {
		return domain.ExecutionResult{}, ErrOperationUnknown
	}

	// Gating obligatorio: side effects requieren aprobación de Review
	if hasSideEffect && spec.ReviewRequestID != nil {
		approved, err := uc.checker.IsApproved(ctx, *spec.ReviewRequestID)
		if err != nil {
			slog.Error("check review approval", "error", err, "review_request_id", spec.ReviewRequestID)
			return domain.ExecutionResult{}, fmt.Errorf("check review approval: %w", err)
		}
		if !approved {
			return domain.ExecutionResult{}, ErrUngated
		}
	} else if hasSideEffect && spec.ReviewRequestID == nil {
		return domain.ExecutionResult{}, ErrUngated
	}

	// Validar spec
	if err := conn.Validate(spec); err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("validate spec: %w", err)
	}

	// Ejecutar
	result, err := conn.Execute(ctx, spec)
	if err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("execute connector %s: %w", conn.ID(), err)
	}

	// Persistir resultado
	if saveErr := uc.repo.SaveExecution(ctx, result); saveErr != nil {
		slog.Error("save execution result", "error", saveErr, "connector", conn.ID())
	}

	return result, nil
}

// ListExecutions lista resultados de ejecución de un conector.
func (uc *Usecases) ListExecutions(ctx context.Context, connectorID uuid.UUID, limit int) ([]domain.ExecutionResult, error) {
	if limit <= 0 {
		limit = 50
	}
	return uc.repo.ListExecutions(ctx, connectorID, limit)
}

// Capabilities lista las capacidades de todos los conectores registrados.
func (uc *Usecases) Capabilities() []ConnectorCapabilities {
	var out []ConnectorCapabilities
	for _, c := range uc.registry.List() {
		out = append(out, ConnectorCapabilities{
			ID:           c.ID(),
			Kind:         c.Kind(),
			Capabilities: c.Capabilities(),
		})
	}
	return out
}

// ConnectorCapabilities agrupa capacidades por conector.
type ConnectorCapabilities struct {
	ID           string
	Kind         string
	Capabilities []domain.Capability
}

// ReviewCheckerAdapter adapta el reviewclient para verificar aprobaciones.
type ReviewCheckerAdapter struct {
	getRequest func(ctx context.Context, id uuid.UUID) (status string, httpStatus int, err error)
}

// NewReviewCheckerAdapter crea un adaptador para verificar aprobaciones.
func NewReviewCheckerAdapter(getRequest func(ctx context.Context, id uuid.UUID) (string, int, error)) *ReviewCheckerAdapter {
	return &ReviewCheckerAdapter{getRequest: getRequest}
}

// IsApproved verifica si un request de Review fue aprobado.
func (a *ReviewCheckerAdapter) IsApproved(ctx context.Context, reviewRequestID uuid.UUID) (bool, error) {
	status, _, err := a.getRequest(ctx, reviewRequestID)
	if err != nil {
		return false, err
	}
	// Estados que indican aprobación
	return status == "allowed" || status == "approved" || status == "executed", nil
}

// SeedDefaultConnectors registra conectores por defecto en el registry y en DB.
func (uc *Usecases) SeedDefaultConnectors(ctx context.Context) error {
	for _, conn := range uc.registry.List() {
		existing, _ := uc.repo.ListConnectors(ctx)
		found := false
		for _, e := range existing {
			if e.Kind == conn.Kind() {
				found = true
				break
			}
		}
		if !found {
			capsJSON, _ := json.Marshal(conn.Capabilities())
			_, err := uc.repo.SaveConnector(ctx, domain.Connector{
				Name:       conn.Kind(),
				Kind:       conn.Kind(),
				Enabled:    true,
				ConfigJSON: json.RawMessage(`{}`),
			})
			if err != nil {
				slog.Error("seed connector", "kind", conn.Kind(), "error", err)
			} else {
				slog.Info("seeded connector", "kind", conn.Kind(), "capabilities", string(capsJSON))
			}
		}
	}
	return nil
}

// ignore para compilación
var _ = time.Now
