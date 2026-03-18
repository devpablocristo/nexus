package approvals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/postgres"
	approvaldomain "github.com/devpablocristo/nexus/v3/review/internal/approvals/usecases/domain"
)

// Sentinel errors
var (
	ErrNotFound   = errors.New("approval not found")
	ErrNotPending = errors.New("approval is not pending")
)

// Repository define el port de persistencia para approvals.
type Repository interface {
	Create(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error)
	GetByID(ctx context.Context, id uuid.UUID) (approvaldomain.Approval, error)
	GetByRequestID(ctx context.Context, requestID uuid.UUID) (*approvaldomain.Approval, error)
	ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error)
	Update(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error)
}

// --- Implementación PostgreSQL ---

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO approvals (id, request_id, status, decided_by, decision_note, decided_at, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, a.ID, a.RequestID, a.Status, a.DecidedBy, a.DecisionNote, a.DecidedAt, a.ExpiresAt, a.CreatedAt)
	if err != nil {
		return approvaldomain.Approval{}, fmt.Errorf("insert approval: %w", err)
	}
	return a, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (approvaldomain.Approval, error) {
	row := r.db.Pool().QueryRow(ctx, selectApprovalSQL+` WHERE id = $1`, id)
	a, err := scanApproval(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approvaldomain.Approval{}, ErrNotFound
		}
		return approvaldomain.Approval{}, err
	}
	return a, nil
}

func (r *PostgresRepository) GetByRequestID(ctx context.Context, requestID uuid.UUID) (*approvaldomain.Approval, error) {
	row := r.db.Pool().QueryRow(ctx, selectApprovalSQL+` WHERE request_id = $1`, requestID)
	a, err := scanApproval(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (r *PostgresRepository) ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error) {
	// Expira en query time: excluir approvals vencidos
	query := selectApprovalSQL + ` WHERE status = 'pending' AND expires_at > now() ORDER BY created_at ASC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := r.db.Pool().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	defer rows.Close()

	out := make([]approvaldomain.Approval, 0)
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error) {
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE approvals SET status = $2, decided_by = $3, decision_note = $4, decided_at = $5
		WHERE id = $1
	`, a.ID, a.Status, a.DecidedBy, a.DecisionNote, a.DecidedAt)
	if err != nil {
		return approvaldomain.Approval{}, fmt.Errorf("update approval: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return approvaldomain.Approval{}, ErrNotFound
	}
	return a, nil
}

// --- Scanner ---

const selectApprovalSQL = `SELECT id, request_id, status, decided_by, decision_note, decided_at, expires_at, created_at FROM approvals`

type approvalScanRow interface {
	Scan(dest ...any) error
}

func scanApproval(row approvalScanRow) (approvaldomain.Approval, error) {
	var a approvaldomain.Approval
	if err := row.Scan(
		&a.ID, &a.RequestID, &a.Status, &a.DecidedBy, &a.DecisionNote, &a.DecidedAt, &a.ExpiresAt, &a.CreatedAt,
	); err != nil {
		return approvaldomain.Approval{}, fmt.Errorf("scan approval: %w", err)
	}
	return a, nil
}
