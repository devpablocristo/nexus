package policies

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	policydomain "nexus/v2/control-plane/internal/policies/usecases/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, func(), error) {
	db, err := sharedpostgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open policies postgres database: %w", err)
	}
	repo, err := NewPostgresRepositoryWithDB(ctx, db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return repo, db.Close, nil
}

func NewPostgresRepositoryWithDB(ctx context.Context, db *sharedpostgres.DB) (*PostgresRepository, error) {
	if err := sharedpostgres.MigrateUp(ctx, db, "control-plane/policies", migrationFiles, "migrations"); err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, item policydomain.Policy) (policydomain.Policy, error) {
	now := time.Now().UTC()
	id := uuid.New()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	item.ID = id.String()

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO action_policies (
			id, action_type, resource_type, effect, priority, expression, reason,
			require_approval, approval_ttl_seconds, enabled, archived_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		id, item.ActionType, item.ResourceType, item.Effect, item.Priority, item.Expression, item.Reason,
		item.RequireApproval, item.ApprovalTTLSeconds, item.Enabled, item.ArchivedAt, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return policydomain.Policy{}, fmt.Errorf("insert policy: %w", err)
	}
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error) {
	query := `
		SELECT id, action_type, resource_type, effect, priority, expression, reason,
		       require_approval, approval_ttl_seconds, enabled, archived_at, created_at, updated_at
		FROM action_policies
		WHERE ($1 = '' OR action_type = $1)
		  AND ($2 = '' OR resource_type = $2)
	`
	args := []any{filters.ActionType, filters.ResourceType}
	if filters.Archived != nil {
		if *filters.Archived {
			query += ` AND archived_at IS NOT NULL`
		} else {
			query += ` AND archived_at IS NULL`
		}
	}
	query += ` ORDER BY priority ASC, created_at ASC, id ASC`

	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	items := make([]policydomain.Policy, 0)
	for rows.Next() {
		item, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policies: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, action_type, resource_type, effect, priority, expression, reason,
		       require_approval, approval_ttl_seconds, enabled, archived_at, created_at, updated_at
		FROM action_policies
		WHERE id = $1
	`, id)
	item, err := scanPolicy(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return policydomain.Policy{}, ErrNotFound
		}
		return policydomain.Policy{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Save(ctx context.Context, item policydomain.Policy) (policydomain.Policy, error) {
	id, err := uuid.Parse(item.ID)
	if err != nil {
		return policydomain.Policy{}, ErrNotFound
	}
	item.UpdatedAt = time.Now().UTC()

	row := r.db.Pool().QueryRow(ctx, `
		UPDATE action_policies
		SET action_type = $2,
			resource_type = $3,
			effect = $4,
			priority = $5,
			expression = $6,
			reason = $7,
			require_approval = $8,
			approval_ttl_seconds = $9,
			enabled = $10,
			updated_at = $11
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, action_type, resource_type, effect, priority, expression, reason,
		          require_approval, approval_ttl_seconds, enabled, archived_at, created_at, updated_at
	`,
		id, item.ActionType, item.ResourceType, item.Effect, item.Priority, item.Expression, item.Reason,
		item.RequireApproval, item.ApprovalTTLSeconds, item.Enabled, item.UpdatedAt,
	)
	updated, err := scanPolicy(row)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return policydomain.Policy{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return policydomain.Policy{}, currentErr
	}
	if current.ArchivedAt != nil {
		return policydomain.Policy{}, ErrArchived
	}
	return policydomain.Policy{}, ErrNotFound
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM action_policies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ArchiveByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	now := time.Now().UTC()
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE action_policies
		SET archived_at = $2,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, action_type, resource_type, effect, priority, expression, reason,
		          require_approval, approval_ttl_seconds, enabled, archived_at, created_at, updated_at
	`, id, now)
	item, err := scanPolicy(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return policydomain.Policy{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return policydomain.Policy{}, currentErr
	}
	if current.ArchivedAt != nil {
		return policydomain.Policy{}, ErrAlreadyArchived
	}
	return policydomain.Policy{}, ErrNotFound
}

func (r *PostgresRepository) RestoreByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	now := time.Now().UTC()
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE action_policies
		SET archived_at = NULL,
			updated_at = $2
		WHERE id = $1 AND archived_at IS NOT NULL
		RETURNING id, action_type, resource_type, effect, priority, expression, reason,
		          require_approval, approval_ttl_seconds, enabled, archived_at, created_at, updated_at
	`, id, now)
	item, err := scanPolicy(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return policydomain.Policy{}, err
	}

	current, currentErr := r.GetByID(ctx, id)
	if currentErr != nil {
		return policydomain.Policy{}, currentErr
	}
	if current.ArchivedAt == nil {
		return policydomain.Policy{}, ErrNotArchived
	}
	return policydomain.Policy{}, ErrNotFound
}

type policyScanRow interface {
	Scan(dest ...any) error
}

func scanPolicy(row policyScanRow) (policydomain.Policy, error) {
	var (
		id                 uuid.UUID
		actionType         string
		resourceType       string
		effect             string
		priority           int
		expression         string
		reason             string
		requireApproval    bool
		approvalTTLSeconds int
		enabled            bool
		archivedAt         *time.Time
		createdAt          time.Time
		updatedAt          time.Time
	)
	if err := row.Scan(
		&id, &actionType, &resourceType, &effect, &priority, &expression, &reason,
		&requireApproval, &approvalTTLSeconds, &enabled, &archivedAt, &createdAt, &updatedAt,
	); err != nil {
		return policydomain.Policy{}, fmt.Errorf("scan policy: %w", err)
	}

	return policydomain.Policy{
		ID:                 id.String(),
		ActionType:         actionType,
		ResourceType:       resourceType,
		Effect:             policydomain.Effect(effect),
		Priority:           priority,
		Expression:         expression,
		Reason:             reason,
		RequireApproval:    requireApproval,
		ApprovalTTLSeconds: approvalTTLSeconds,
		Enabled:            enabled,
		ArchivedAt:         archivedAt,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}
