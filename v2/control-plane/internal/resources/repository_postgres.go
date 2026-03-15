package resources

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	resourcedomain "nexus/v2/control-plane/internal/resources/usecases/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, func(), error) {
	db, err := sharedpostgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open resources postgres database: %w", err)
	}
	repo, err := NewPostgresRepositoryWithDB(ctx, db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return repo, db.Close, nil
}

func NewPostgresRepositoryWithDB(ctx context.Context, db *sharedpostgres.DB) (*PostgresRepository, error) {
	if err := sharedpostgres.MigrateUp(ctx, db, "control-plane/resources", migrationFiles, "migrations"); err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, item resourcedomain.ProtectedResource) (resourcedomain.ProtectedResource, error) {
	now := time.Now().UTC()
	id := uuid.New()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	item.ID = id.String()

	labels := cloneLabels(item.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	payload, err := json.Marshal(labels)
	if err != nil {
		return resourcedomain.ProtectedResource{}, fmt.Errorf("marshal resource labels: %w", err)
	}

	_, err = r.db.Pool().Exec(ctx, `
		INSERT INTO protected_resources (
			id, type, name, environment, chain, labels, criticality, archived_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, id, item.Type, item.Name, item.Environment, item.Chain, payload, item.Criticality, item.ArchivedAt, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return resourcedomain.ProtectedResource{}, fmt.Errorf("insert resource: %w", err)
	}

	item.Labels = labels
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]resourcedomain.ProtectedResource, error) {
	query := strings.Builder{}
	query.WriteString(`
		SELECT id, type, name, environment, chain, labels, criticality, archived_at, created_at, updated_at
		FROM protected_resources
		WHERE 1=1
	`)
	args := make([]any, 0, 4)

	addClause := func(clause string, value any) {
		args = append(args, value)
		_, _ = fmt.Fprintf(&query, " AND %s $%d", clause, len(args))
	}

	if filters.Type != "" {
		addClause("type =", filters.Type)
	}
	if filters.Environment != "" {
		addClause("environment =", filters.Environment)
	}
	if filters.Archived != nil {
		if *filters.Archived {
			query.WriteString(" AND archived_at IS NOT NULL")
		} else {
			query.WriteString(" AND archived_at IS NULL")
		}
	}
	query.WriteString(" ORDER BY created_at DESC, id DESC")
	if filters.Limit > 0 {
		args = append(args, filters.Limit)
		_, _ = fmt.Fprintf(&query, " LIMIT $%d", len(args))
	}

	rows, err := r.db.Pool().Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	items := make([]resourcedomain.ProtectedResource, 0)
	for rows.Next() {
		item, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resources: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, type, name, environment, chain, labels, criticality, archived_at, created_at, updated_at
		FROM protected_resources
		WHERE id = $1
	`, id)
	item, err := scanResource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return resourcedomain.ProtectedResource{}, ErrNotFound
		}
		return resourcedomain.ProtectedResource{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Update(ctx context.Context, item resourcedomain.ProtectedResource) (resourcedomain.ProtectedResource, error) {
	id, err := uuid.Parse(item.ID)
	if err != nil {
		return resourcedomain.ProtectedResource{}, ErrNotFound
	}
	labels := cloneLabels(item.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	payload, err := json.Marshal(labels)
	if err != nil {
		return resourcedomain.ProtectedResource{}, fmt.Errorf("marshal resource labels: %w", err)
	}
	item.UpdatedAt = time.Now().UTC()

	row := r.db.Pool().QueryRow(ctx, `
		UPDATE protected_resources
		SET type = $2,
			name = $3,
			environment = $4,
			chain = $5,
			labels = $6,
			criticality = $7,
			updated_at = $8
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, type, name, environment, chain, labels, criticality, archived_at, created_at, updated_at
	`, id, item.Type, item.Name, item.Environment, item.Chain, payload, item.Criticality, item.UpdatedAt)

	updated, err := scanResource(row)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return resourcedomain.ProtectedResource{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return resourcedomain.ProtectedResource{}, currentErr
	}
	if current.ArchivedAt != nil {
		return resourcedomain.ProtectedResource{}, ErrArchived
	}
	return resourcedomain.ProtectedResource{}, ErrNotFound
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM protected_resources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (resourcedomain.ProtectedResource, error) {
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE protected_resources
		SET archived_at = $2,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, type, name, environment, chain, labels, criticality, archived_at, created_at, updated_at
	`, id, archivedAt)
	item, err := scanResource(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return resourcedomain.ProtectedResource{}, err
	}
	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return resourcedomain.ProtectedResource{}, currentErr
	}
	if current.ArchivedAt != nil {
		return resourcedomain.ProtectedResource{}, ErrAlreadyArchived
	}
	return resourcedomain.ProtectedResource{}, ErrNotFound
}

func (r *PostgresRepository) Restore(ctx context.Context, id uuid.UUID, restoredAt time.Time) (resourcedomain.ProtectedResource, error) {
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE protected_resources
		SET archived_at = NULL,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NOT NULL
		RETURNING id, type, name, environment, chain, labels, criticality, archived_at, created_at, updated_at
	`, id, restoredAt)
	item, err := scanResource(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return resourcedomain.ProtectedResource{}, err
	}
	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return resourcedomain.ProtectedResource{}, currentErr
	}
	if current.ArchivedAt == nil {
		return resourcedomain.ProtectedResource{}, ErrNotArchived
	}
	return resourcedomain.ProtectedResource{}, ErrNotFound
}

type resourceScanRow interface {
	Scan(dest ...any) error
}

func scanResource(row resourceScanRow) (resourcedomain.ProtectedResource, error) {
	var (
		id           uuid.UUID
		resourceType string
		name         string
		environment  string
		chain        string
		labelsRaw    []byte
		criticality  string
		archivedAt   *time.Time
		createdAt    time.Time
		updatedAt    time.Time
	)
	if err := row.Scan(&id, &resourceType, &name, &environment, &chain, &labelsRaw, &criticality, &archivedAt, &createdAt, &updatedAt); err != nil {
		return resourcedomain.ProtectedResource{}, fmt.Errorf("scan resource: %w", err)
	}

	labels := map[string]string{}
	if len(labelsRaw) > 0 {
		if err := json.Unmarshal(labelsRaw, &labels); err != nil {
			return resourcedomain.ProtectedResource{}, fmt.Errorf("decode resource labels: %w", err)
		}
	}

	return resourcedomain.ProtectedResource{
		ID:          id.String(),
		Type:        resourcedomain.ResourceType(resourceType),
		Name:        name,
		Environment: environment,
		Chain:       chain,
		Labels:      labels,
		Criticality: resourcedomain.Criticality(criticality),
		ArchivedAt:  archivedAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}
