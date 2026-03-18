package policies

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	policydomain "github.com/devpablocristo/nexus/review-v1/internal/policies/usecases/domain"
)

// Sentinel errors
var (
	ErrNotFound      = errors.New("policy not found")
	ErrAlreadyExists = errors.New("policy already exists")
	ErrArchived      = errors.New("policy is archived")
)

// ListFilters define los filtros para listar políticas.
type ListFilters struct {
	IncludeArchived bool
	EnabledOnly     bool
}

// Repository define el port de persistencia para políticas.
type Repository interface {
	Create(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error)
	GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error)
	Update(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) error
	RestoreByID(ctx context.Context, id uuid.UUID) error
}

// --- Implementación PostgreSQL ---

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const selectPolicySQL = `
	SELECT id, name, description, action_type, target_system,
	       expression, effect, risk_override, priority, origin, proposal_id,
	       enabled, archived_at, created_at, updated_at
	FROM policies`

func (r *PostgresRepository) Create(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error) {
	now := time.Now().UTC()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO policies (
			id, name, description, action_type, target_system,
			expression, effect, risk_override, priority, origin, proposal_id,
			enabled, archived_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`,
		p.ID, p.Name, p.Description, p.ActionType, p.TargetSystem,
		p.Expression, p.Effect, p.RiskOverride, p.Priority, p.Origin, p.ProposalID,
		p.Enabled, p.ArchivedAt, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return policydomain.Policy{}, fmt.Errorf("insert policy: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	row := r.db.Pool().QueryRow(ctx, selectPolicySQL+` WHERE id = $1`, id)
	p, err := scanPolicy(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return policydomain.Policy{}, ErrNotFound
		}
		return policydomain.Policy{}, fmt.Errorf("get policy: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error) {
	query := selectPolicySQL + ` WHERE 1=1`
	args := []any{}
	argN := 1

	if !filters.IncludeArchived {
		query += ` AND archived_at IS NULL`
	}
	if filters.EnabledOnly {
		query += fmt.Sprintf(` AND enabled = $%d`, argN)
		args = append(args, true)
		argN++
	}
	query += ` ORDER BY priority ASC, created_at DESC`

	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	out := make([]policydomain.Policy, 0)
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error) {
	p.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE policies SET
			name = $2, description = $3, action_type = $4, target_system = $5,
			expression = $6, effect = $7, risk_override = $8, priority = $9,
			enabled = $10, updated_at = $11
		WHERE id = $1
	`,
		p.ID, p.Name, p.Description, p.ActionType, p.TargetSystem,
		p.Expression, p.Effect, p.RiskOverride, p.Priority,
		p.Enabled, p.UpdatedAt,
	)
	if err != nil {
		return policydomain.Policy{}, fmt.Errorf("update policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return policydomain.Policy{}, ErrNotFound
	}
	return p, nil
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM policies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ArchiveByID(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE policies SET archived_at = now(), updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("archive policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) RestoreByID(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE policies SET archived_at = NULL, updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("restore policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Scanner ---

type policyScanRow interface {
	Scan(dest ...any) error
}

func scanPolicy(row policyScanRow) (policydomain.Policy, error) {
	var p policydomain.Policy
	if err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.ActionType, &p.TargetSystem,
		&p.Expression, &p.Effect, &p.RiskOverride, &p.Priority, &p.Origin, &p.ProposalID,
		&p.Enabled, &p.ArchivedAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return policydomain.Policy{}, fmt.Errorf("scan policy: %w", err)
	}
	return p, nil
}
