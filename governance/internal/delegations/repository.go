package delegations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	domain "github.com/devpablocristo/nexus/governance/internal/delegations/usecases/domain"
)

var (
	ErrNotFound = domainerr.NotFound("not found")
)

type Repository interface {
	Create(ctx context.Context, d domain.Delegation) (domain.Delegation, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Delegation, error)
	ListByAgentID(ctx context.Context, agentID string) ([]domain.Delegation, error)
	List(ctx context.Context) ([]domain.Delegation, error)
	Update(ctx context.Context, d domain.Delegation) (domain.Delegation, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const selectSQL = `SELECT id, org_id, owner_id, owner_type, agent_id, agent_type, allowed_action_types, allowed_resources, purpose, max_risk_class, expires_at, enabled, created_at, updated_at FROM delegations`

func (r *PostgresRepository) Create(ctx context.Context, d domain.Delegation) (domain.Delegation, error) {
	now := time.Now().UTC()
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.CreatedAt = now
	d.UpdatedAt = now

	atJSON, _ := json.Marshal(d.AllowedActionTypes)
	arJSON, _ := json.Marshal(d.AllowedResources)

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO delegations (id, org_id, owner_id, owner_type, agent_id, agent_type, allowed_action_types, allowed_resources, purpose, max_risk_class, expires_at, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	`, d.ID, d.OrgID, d.OwnerID, d.OwnerType, d.AgentID, d.AgentType, atJSON, arJSON, d.Purpose, d.MaxRiskClass, d.ExpiresAt, d.Enabled, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return domain.Delegation{}, fmt.Errorf("insert delegation: %w", err)
	}
	return d, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Delegation, error) {
	row := r.db.Pool().QueryRow(ctx, selectSQL+` WHERE id = $1`, id)
	d, err := scanDelegation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Delegation{}, ErrNotFound
		}
		return domain.Delegation{}, err
	}
	return d, nil
}

func (r *PostgresRepository) ListByAgentID(ctx context.Context, agentID string) ([]domain.Delegation, error) {
	rows, err := r.db.Pool().Query(ctx, selectSQL+` WHERE agent_id = $1 ORDER BY created_at DESC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("list delegations by agent: %w", err)
	}
	defer rows.Close()
	return scanDelegations(rows)
}

func (r *PostgresRepository) List(ctx context.Context) ([]domain.Delegation, error) {
	rows, err := r.db.Pool().Query(ctx, selectSQL+` ORDER BY owner_id, agent_id`)
	if err != nil {
		return nil, fmt.Errorf("list delegations: %w", err)
	}
	defer rows.Close()
	return scanDelegations(rows)
}

func (r *PostgresRepository) Update(ctx context.Context, d domain.Delegation) (domain.Delegation, error) {
	d.UpdatedAt = time.Now().UTC()
	atJSON, _ := json.Marshal(d.AllowedActionTypes)
	arJSON, _ := json.Marshal(d.AllowedResources)

	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE delegations SET
			owner_id = $2, owner_type = $3, agent_id = $4, agent_type = $5,
			allowed_action_types = $6, allowed_resources = $7, purpose = $8,
			max_risk_class = $9, expires_at = $10, enabled = $11, updated_at = $12
		WHERE id = $1
	`, d.ID, d.OwnerID, d.OwnerType, d.AgentID, d.AgentType, atJSON, arJSON, d.Purpose, d.MaxRiskClass, d.ExpiresAt, d.Enabled, d.UpdatedAt)
	if err != nil {
		return domain.Delegation{}, fmt.Errorf("update delegation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Delegation{}, ErrNotFound
	}
	return d, nil
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM delegations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete delegation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanRow interface {
	Scan(dest ...any) error
}

func scanDelegation(row scanRow) (domain.Delegation, error) {
	var d domain.Delegation
	var atJSON, arJSON []byte
	if err := row.Scan(
		&d.ID, &d.OrgID, &d.OwnerID, &d.OwnerType, &d.AgentID, &d.AgentType,
		&atJSON, &arJSON, &d.Purpose, &d.MaxRiskClass, &d.ExpiresAt,
		&d.Enabled, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return domain.Delegation{}, fmt.Errorf("scan delegation: %w", err)
	}
	json.Unmarshal(atJSON, &d.AllowedActionTypes)
	json.Unmarshal(arJSON, &d.AllowedResources)
	return d, nil
}

func scanDelegations(rows interface {
	Next() bool
	Err() error
	scanRow
}) ([]domain.Delegation, error) {
	out := make([]domain.Delegation, 0)
	for rows.Next() {
		d, err := scanDelegation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
