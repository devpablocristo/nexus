package actiontypes

import (
	"context"
	"errors"

	"github.com/devpablocristo/core/backend/go/domainerr"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	domain "github.com/devpablocristo/nexus/v3/review/internal/actiontypes/usecases/domain"
)

var (
	ErrNotFound = domainerr.NotFound("not found")
	ErrAlreadyExists = errors.New("action type already exists")
)

type Repository interface {
	Create(ctx context.Context, at domain.ActionType) (domain.ActionType, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.ActionType, error)
	GetByName(ctx context.Context, name string) (domain.ActionType, error)
	List(ctx context.Context) ([]domain.ActionType, error)
	Update(ctx context.Context, at domain.ActionType) (domain.ActionType, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const selectSQL = `SELECT id, name, description, category, risk_class, schema, reversible, requires_break_glass, enabled, created_at, updated_at FROM action_types`

func (r *PostgresRepository) Create(ctx context.Context, at domain.ActionType) (domain.ActionType, error) {
	now := time.Now().UTC()
	if at.ID == uuid.Nil {
		at.ID = uuid.New()
	}
	at.CreatedAt = now
	at.UpdatedAt = now

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO action_types (id, name, description, category, risk_class, schema, reversible, requires_break_glass, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, at.ID, at.Name, at.Description, at.Category, at.RiskClass, at.Schema, at.Reversible, at.RequiresBreakGlass, at.Enabled, at.CreatedAt, at.UpdatedAt)
	if err != nil {
		return domain.ActionType{}, fmt.Errorf("insert action type: %w", err)
	}
	return at, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.ActionType, error) {
	row := r.db.Pool().QueryRow(ctx, selectSQL+` WHERE id = $1`, id)
	at, err := scanActionType(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ActionType{}, ErrNotFound
		}
		return domain.ActionType{}, err
	}
	return at, nil
}

func (r *PostgresRepository) GetByName(ctx context.Context, name string) (domain.ActionType, error) {
	row := r.db.Pool().QueryRow(ctx, selectSQL+` WHERE name = $1`, name)
	at, err := scanActionType(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ActionType{}, ErrNotFound
		}
		return domain.ActionType{}, err
	}
	return at, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]domain.ActionType, error) {
	rows, err := r.db.Pool().Query(ctx, selectSQL+` ORDER BY category, name`)
	if err != nil {
		return nil, fmt.Errorf("list action types: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ActionType, 0)
	for rows.Next() {
		at, err := scanActionType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, at)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, at domain.ActionType) (domain.ActionType, error) {
	at.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE action_types SET
			name = $2, description = $3, category = $4, risk_class = $5,
			schema = $6, reversible = $7, requires_break_glass = $8,
			enabled = $9, updated_at = $10
		WHERE id = $1
	`, at.ID, at.Name, at.Description, at.Category, at.RiskClass, at.Schema, at.Reversible, at.RequiresBreakGlass, at.Enabled, at.UpdatedAt)
	if err != nil {
		return domain.ActionType{}, fmt.Errorf("update action type: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ActionType{}, ErrNotFound
	}
	return at, nil
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM action_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete action type: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanRow interface {
	Scan(dest ...any) error
}

func scanActionType(row scanRow) (domain.ActionType, error) {
	var at domain.ActionType
	if err := row.Scan(
		&at.ID, &at.Name, &at.Description, &at.Category, &at.RiskClass,
		&at.Schema, &at.Reversible, &at.RequiresBreakGlass, &at.Enabled,
		&at.CreatedAt, &at.UpdatedAt,
	); err != nil {
		return domain.ActionType{}, fmt.Errorf("scan action type: %w", err)
	}
	return at, nil
}
