package incidents

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	incidentdomain "nexus/v2/control-workers/internal/incidents/usecases/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, func(), error) {
	db, err := sharedpostgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open incidents postgres database: %w", err)
	}
	repo, err := NewPostgresRepositoryWithDB(ctx, db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return repo, db.Close, nil
}

func NewPostgresRepositoryWithDB(ctx context.Context, db *sharedpostgres.DB) (*PostgresRepository, error) {
	if err := sharedpostgres.MigrateUp(ctx, db, "control-workers/incidents", migrationFiles, "migrations"); err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, item incidentdomain.Incident) (incidentdomain.Incident, error) {
	now := time.Now().UTC()
	id := uuid.New()
	item.ID = id.String()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	details, err := marshalIncidentDetails(item.Details)
	if err != nil {
		return incidentdomain.Incident{}, fmt.Errorf("marshal incident details: %w", err)
	}

	_, err = r.db.Pool().Exec(ctx, `
		INSERT INTO incidents (
			id, source_kind, source_id, action_type, resource_id, resource_type, trigger, risk_level, severity,
			status, summary, reason, details, archived_at, resolved_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`,
		id, item.SourceKind, item.SourceID, item.ActionType, item.ResourceID, item.ResourceType, item.Trigger, item.RiskLevel, item.Severity,
		item.Status, item.Summary, item.Reason, details, item.ArchivedAt, item.ResolvedAt, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return incidentdomain.Incident{}, fmt.Errorf("insert incident: %w", err)
	}
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]incidentdomain.Incident, error) {
	query := `
		SELECT id, source_kind, source_id, action_type, resource_id, resource_type, trigger, risk_level, severity,
		       status, summary, reason, details, archived_at, resolved_at, created_at, updated_at
		FROM incidents
		WHERE ($1 = '' OR source_kind = $1)
		  AND ($2 = '' OR trigger = $2)
		  AND ($3 = '' OR severity = $3)
		  AND ($4 = '' OR status = $4)
	`
	args := []any{filters.SourceKind, filters.Trigger, filters.Severity, filters.Status}
	if filters.Archived != nil {
		if *filters.Archived {
			query += ` AND archived_at IS NOT NULL`
		} else {
			query += ` AND archived_at IS NULL`
		}
	}
	query += ` ORDER BY created_at DESC, id DESC`
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, filters.Limit)
	}

	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	items := make([]incidentdomain.Incident, 0)
	for rows.Next() {
		item, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incidents: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, source_kind, source_id, action_type, resource_id, resource_type, trigger, risk_level, severity,
		       status, summary, reason, details, archived_at, resolved_at, created_at, updated_at
		FROM incidents
		WHERE id = $1
	`, id)
	item, err := scanIncident(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return incidentdomain.Incident{}, ErrNotFound
		}
		return incidentdomain.Incident{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Update(ctx context.Context, item incidentdomain.Incident) (incidentdomain.Incident, error) {
	id, err := uuid.Parse(item.ID)
	if err != nil {
		return incidentdomain.Incident{}, ErrNotFound
	}
	details, err := marshalIncidentDetails(item.Details)
	if err != nil {
		return incidentdomain.Incident{}, fmt.Errorf("marshal incident details: %w", err)
	}
	item.UpdatedAt = time.Now().UTC()

	row := r.db.Pool().QueryRow(ctx, `
		UPDATE incidents
		SET source_kind = $2,
			source_id = $3,
			action_type = $4,
			resource_id = $5,
			resource_type = $6,
			trigger = $7,
			risk_level = $8,
			severity = $9,
			status = $10,
			summary = $11,
			reason = $12,
			details = $13,
			resolved_at = $14,
			updated_at = $15
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, source_kind, source_id, action_type, resource_id, resource_type, trigger, risk_level, severity,
		          status, summary, reason, details, archived_at, resolved_at, created_at, updated_at
	`,
		id, item.SourceKind, item.SourceID, item.ActionType, item.ResourceID, item.ResourceType, item.Trigger, item.RiskLevel, item.Severity,
		item.Status, item.Summary, item.Reason, details, item.ResolvedAt, item.UpdatedAt,
	)
	updated, err := scanIncident(row)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return incidentdomain.Incident{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return incidentdomain.Incident{}, currentErr
	}
	if current.ArchivedAt != nil {
		return incidentdomain.Incident{}, ErrArchived
	}
	return incidentdomain.Incident{}, ErrNotFound
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM incidents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete incident: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (incidentdomain.Incident, error) {
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE incidents
		SET archived_at = $2,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, source_kind, source_id, action_type, resource_id, resource_type, trigger, risk_level, severity,
		          status, summary, reason, details, archived_at, resolved_at, created_at, updated_at
	`, id, archivedAt)
	item, err := scanIncident(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return incidentdomain.Incident{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return incidentdomain.Incident{}, currentErr
	}
	if current.ArchivedAt != nil {
		return incidentdomain.Incident{}, ErrAlreadyArchived
	}
	return incidentdomain.Incident{}, ErrNotFound
}

func (r *PostgresRepository) Restore(ctx context.Context, id uuid.UUID, restoredAt time.Time) (incidentdomain.Incident, error) {
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE incidents
		SET archived_at = NULL,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NOT NULL
		RETURNING id, source_kind, source_id, action_type, resource_id, resource_type, trigger, risk_level, severity,
		          status, summary, reason, details, archived_at, resolved_at, created_at, updated_at
	`, id, restoredAt)
	item, err := scanIncident(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return incidentdomain.Incident{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return incidentdomain.Incident{}, currentErr
	}
	if current.ArchivedAt == nil {
		return incidentdomain.Incident{}, ErrNotArchived
	}
	return incidentdomain.Incident{}, ErrNotFound
}

type incidentScanRow interface{ Scan(dest ...any) error }

func scanIncident(row incidentScanRow) (incidentdomain.Incident, error) {
	var (
		item       incidentdomain.Incident
		details    []byte
		archivedAt *time.Time
		resolvedAt *time.Time
	)
	if err := row.Scan(
		&item.ID, &item.SourceKind, &item.SourceID, &item.ActionType, &item.ResourceID, &item.ResourceType, &item.Trigger, &item.RiskLevel, &item.Severity,
		&item.Status, &item.Summary, &item.Reason, &details, &archivedAt, &resolvedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return incidentdomain.Incident{}, fmt.Errorf("scan incident: %w", err)
	}
	item.ArchivedAt = archivedAt
	item.ResolvedAt = resolvedAt
	if err := unmarshalIncidentDetails(details, &item.Details); err != nil {
		return incidentdomain.Incident{}, fmt.Errorf("decode incident details: %w", err)
	}
	return item, nil
}

func marshalIncidentDetails(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte(`{}`), nil
	}
	return json.Marshal(value)
}

func unmarshalIncidentDetails(raw []byte, out *map[string]any) error {
	if len(raw) == 0 {
		*out = map[string]any{}
		return nil
	}
	return json.Unmarshal(raw, out)
}
