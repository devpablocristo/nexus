package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/connectors/usecases/domain"
)

// PostgresRepository implementación PostgreSQL del repositorio de conectores.
type PostgresRepository struct {
	db *sharedpostgres.DB
}

// NewPostgresRepository crea un nuevo repositorio de conectores.
func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// SaveConnector crea un nuevo conector.
func (r *PostgresRepository) SaveConnector(ctx context.Context, c domain.Connector) (domain.Connector, error) {
	now := time.Now().UTC()
	c.ID = uuid.New()
	c.CreatedAt = now
	c.UpdatedAt = now
	if len(c.ConfigJSON) == 0 {
		c.ConfigJSON = json.RawMessage(`{}`)
	}

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO companion_connectors (id, name, kind, enabled, config_json, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, c.ID, c.Name, c.Kind, c.Enabled, c.ConfigJSON, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return domain.Connector{}, fmt.Errorf("insert connector: %w", err)
	}
	return c, nil
}

// GetConnector obtiene un conector por ID.
func (r *PostgresRepository) GetConnector(ctx context.Context, id uuid.UUID) (domain.Connector, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, name, kind, enabled, config_json, created_at, updated_at
		FROM companion_connectors WHERE id = $1
	`, id)
	c, err := scanConnector(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Connector{}, ErrNotFound
		}
		return domain.Connector{}, fmt.Errorf("get connector: %w", err)
	}
	return c, nil
}

// ListConnectors lista todos los conectores.
func (r *PostgresRepository) ListConnectors(ctx context.Context) ([]domain.Connector, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, name, kind, enabled, config_json, created_at, updated_at
		FROM companion_connectors ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}
	defer rows.Close()

	var out []domain.Connector
	for rows.Next() {
		c, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateConnector actualiza un conector.
func (r *PostgresRepository) UpdateConnector(ctx context.Context, c domain.Connector) (domain.Connector, error) {
	c.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE companion_connectors
		SET name = $2, enabled = $3, config_json = $4, updated_at = $5
		WHERE id = $1
	`, c.ID, c.Name, c.Enabled, c.ConfigJSON, c.UpdatedAt)
	if err != nil {
		return domain.Connector{}, fmt.Errorf("update connector: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Connector{}, ErrNotFound
	}
	return c, nil
}

// DeleteConnector elimina un conector.
func (r *PostgresRepository) DeleteConnector(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM companion_connectors WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete connector: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SaveExecution persiste un resultado de ejecución.
func (r *PostgresRepository) SaveExecution(ctx context.Context, e domain.ExecutionResult) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if len(e.ResultJSON) == 0 {
		e.ResultJSON = json.RawMessage(`{}`)
	}
	if len(e.Payload) == 0 {
		e.Payload = json.RawMessage(`{}`)
	}

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO companion_connector_executions
			(id, connector_id, operation, status, external_ref, payload, result_json,
			 error_message, retryable, duration_ms, task_id, review_request_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, e.ID, e.ConnectorID, e.Operation, e.Status, e.ExternalRef,
		e.Payload, e.ResultJSON, nullIfEmpty(e.ErrorMessage),
		e.Retryable, e.DurationMS, e.TaskID, e.ReviewRequestID, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("save execution: %w", err)
	}
	return nil
}

// ListExecutions lista resultados de ejecución de un conector.
func (r *PostgresRepository) ListExecutions(ctx context.Context, connectorID uuid.UUID, limit int) ([]domain.ExecutionResult, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, connector_id, operation, status, external_ref, payload, result_json,
		       error_message, retryable, duration_ms, task_id, review_request_id, created_at
		FROM companion_connector_executions
		WHERE connector_id = $1
		ORDER BY created_at DESC LIMIT $2
	`, connectorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()

	var out []domain.ExecutionResult
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanConnector(row rowScanner) (domain.Connector, error) {
	var c domain.Connector
	var configRaw []byte
	err := row.Scan(&c.ID, &c.Name, &c.Kind, &c.Enabled, &configRaw, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return domain.Connector{}, err
	}
	if configRaw != nil {
		c.ConfigJSON = json.RawMessage(configRaw)
	}
	return c, nil
}

func scanExecution(row rowScanner) (domain.ExecutionResult, error) {
	var e domain.ExecutionResult
	var payloadRaw, resultRaw []byte
	var errMsg *string

	err := row.Scan(
		&e.ID, &e.ConnectorID, &e.Operation, &e.Status, &e.ExternalRef,
		&payloadRaw, &resultRaw, &errMsg, &e.Retryable, &e.DurationMS,
		&e.TaskID, &e.ReviewRequestID, &e.CreatedAt,
	)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	if payloadRaw != nil {
		e.Payload = json.RawMessage(payloadRaw)
	}
	if resultRaw != nil {
		e.ResultJSON = json.RawMessage(resultRaw)
	}
	if errMsg != nil {
		e.ErrorMessage = *errMsg
	}
	return e, nil
}
