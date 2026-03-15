package alerts

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
	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, func(), error) {
	db, err := sharedpostgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open alerts postgres database: %w", err)
	}
	repo, err := NewPostgresRepositoryWithDB(ctx, db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return repo, db.Close, nil
}

func NewPostgresRepositoryWithDB(ctx context.Context, db *sharedpostgres.DB) (*PostgresRepository, error) {
	if err := sharedpostgres.MigrateUp(ctx, db, "control-workers/alerts", migrationFiles, "migrations"); err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, item alertdomain.Alert) (alertdomain.Alert, error) {
	now := time.Now().UTC()
	id := uuid.New()
	item.ID = id.String()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	details, err := marshalAlertDetails(item.Details)
	if err != nil {
		return alertdomain.Alert{}, fmt.Errorf("marshal alert details: %w", err)
	}

	_, err = r.db.Pool().Exec(ctx, `
		INSERT INTO alerts (
			id, source_kind, source_id, channel, route, severity, status, summary, body, details, archived_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		id, item.SourceKind, item.SourceID, item.Channel, item.Route, item.Severity, item.Status, item.Summary, item.Body, details, item.ArchivedAt, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return alertdomain.Alert{}, fmt.Errorf("insert alert: %w", err)
	}
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]alertdomain.Alert, error) {
	query := `
		SELECT id, source_kind, source_id, channel, route, severity, status, summary, body, details, archived_at, created_at, updated_at
		FROM alerts
		WHERE ($1 = '' OR source_kind = $1)
		  AND ($2 = '' OR channel = $2)
		  AND ($3 = '' OR severity = $3)
		  AND ($4 = '' OR status = $4)
	`
	args := []any{filters.SourceKind, filters.Channel, filters.Severity, filters.Status}
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
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	items := make([]alertdomain.Alert, 0)
	for rows.Next() {
		item, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alerts: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, source_kind, source_id, channel, route, severity, status, summary, body, details, archived_at, created_at, updated_at
		FROM alerts
		WHERE id = $1
	`, id)
	item, err := scanAlert(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return alertdomain.Alert{}, ErrNotFound
		}
		return alertdomain.Alert{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Update(ctx context.Context, item alertdomain.Alert) (alertdomain.Alert, error) {
	id, err := uuid.Parse(item.ID)
	if err != nil {
		return alertdomain.Alert{}, ErrNotFound
	}
	details, err := marshalAlertDetails(item.Details)
	if err != nil {
		return alertdomain.Alert{}, fmt.Errorf("marshal alert details: %w", err)
	}
	item.UpdatedAt = time.Now().UTC()

	row := r.db.Pool().QueryRow(ctx, `
		UPDATE alerts
		SET source_kind = $2,
			source_id = $3,
			channel = $4,
			route = $5,
			severity = $6,
			status = $7,
			summary = $8,
			body = $9,
			details = $10,
			updated_at = $11
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, source_kind, source_id, channel, route, severity, status, summary, body, details, archived_at, created_at, updated_at
	`, id, item.SourceKind, item.SourceID, item.Channel, item.Route, item.Severity, item.Status, item.Summary, item.Body, details, item.UpdatedAt)
	updated, err := scanAlert(row)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return alertdomain.Alert{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return alertdomain.Alert{}, currentErr
	}
	if current.ArchivedAt != nil {
		return alertdomain.Alert{}, ErrArchived
	}
	return alertdomain.Alert{}, ErrNotFound
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM alerts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete alert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (alertdomain.Alert, error) {
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE alerts
		SET archived_at = $2,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, source_kind, source_id, channel, route, severity, status, summary, body, details, archived_at, created_at, updated_at
	`, id, archivedAt)
	item, err := scanAlert(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return alertdomain.Alert{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return alertdomain.Alert{}, currentErr
	}
	if current.ArchivedAt != nil {
		return alertdomain.Alert{}, ErrAlreadyArchived
	}
	return alertdomain.Alert{}, ErrNotFound
}

func (r *PostgresRepository) Restore(ctx context.Context, id uuid.UUID, restoredAt time.Time) (alertdomain.Alert, error) {
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE alerts
		SET archived_at = NULL,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NOT NULL
		RETURNING id, source_kind, source_id, channel, route, severity, status, summary, body, details, archived_at, created_at, updated_at
	`, id, restoredAt)
	item, err := scanAlert(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return alertdomain.Alert{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return alertdomain.Alert{}, currentErr
	}
	if current.ArchivedAt == nil {
		return alertdomain.Alert{}, ErrNotArchived
	}
	return alertdomain.Alert{}, ErrNotFound
}

type alertScanRow interface{ Scan(dest ...any) error }

func scanAlert(row alertScanRow) (alertdomain.Alert, error) {
	var (
		item       alertdomain.Alert
		details    []byte
		archivedAt *time.Time
	)
	if err := row.Scan(&item.ID, &item.SourceKind, &item.SourceID, &item.Channel, &item.Route, &item.Severity, &item.Status, &item.Summary, &item.Body, &details, &archivedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return alertdomain.Alert{}, fmt.Errorf("scan alert: %w", err)
	}
	item.ArchivedAt = archivedAt
	if err := unmarshalAlertDetails(details, &item.Details); err != nil {
		return alertdomain.Alert{}, fmt.Errorf("decode alert details: %w", err)
	}
	return item, nil
}

func marshalAlertDetails(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte(`{}`), nil
	}
	return json.Marshal(value)
}

func unmarshalAlertDetails(raw []byte, out *map[string]any) error {
	if len(raw) == 0 {
		*out = map[string]any{}
		return nil
	}
	return json.Unmarshal(raw, out)
}
