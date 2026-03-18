package learning

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/postgres"
	learningdomain "github.com/devpablocristo/nexus/v3/review/internal/learning/usecases/domain"
)

// Sentinel errors
var (
	ErrNotFound   = errors.New("proposal not found")
	ErrNotPending = errors.New("proposal is not pending")
)

// Repository define el port de persistencia para policy proposals.
type Repository interface {
	CreateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error)
	ListPendingProposals(ctx context.Context, limit int) ([]learningdomain.PolicyProposal, error)
	GetProposalByID(ctx context.Context, id uuid.UUID) (learningdomain.PolicyProposal, error)
	UpdateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error)
}

// PatternAnalyzer detecta patrones de decisiones para proponer políticas.
type PatternAnalyzer interface {
	Analyze(ctx context.Context, timeWindowDays int, minSampleSize int, minApprovalRate float64) ([]Pattern, error)
}

type Pattern struct {
	ActionType   string
	Total        int
	Approved     int
	ApprovalRate float64
	TimeWindow   string
}

// PolicyCreator crea una policy a partir de un proposal aceptado (inyectado desde wire).
type PolicyCreator interface {
	CreateFromProposal(ctx context.Context, p learningdomain.PolicyProposal) (policyID uuid.UUID, err error)
}

// --- Implementación PostgreSQL ---

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO policy_proposals (
			id, proposed_name, proposed_description, proposed_expression, proposed_effect,
			proposed_action_type, proposed_priority,
			pattern_summary, confidence, sample_size, time_window,
			status, decided_by, decided_at, policy_id, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`,
		p.ID, p.ProposedName, p.ProposedDescription, p.ProposedExpression, p.ProposedEffect,
		p.ProposedActionType, p.ProposedPriority,
		p.PatternSummary, p.Confidence, p.SampleSize, p.TimeWindow,
		p.Status, p.DecidedBy, p.DecidedAt, p.PolicyID, p.CreatedAt,
	)
	if err != nil {
		return learningdomain.PolicyProposal{}, fmt.Errorf("insert proposal: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) ListPendingProposals(ctx context.Context, limit int) ([]learningdomain.PolicyProposal, error) {
	query := selectProposalSQL + ` WHERE status = 'pending' ORDER BY created_at ASC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := r.db.Pool().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list pending proposals: %w", err)
	}
	defer rows.Close()

	out := make([]learningdomain.PolicyProposal, 0)
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) GetProposalByID(ctx context.Context, id uuid.UUID) (learningdomain.PolicyProposal, error) {
	row := r.db.Pool().QueryRow(ctx, selectProposalSQL+` WHERE id = $1`, id)
	p, err := scanProposal(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return learningdomain.PolicyProposal{}, ErrNotFound
		}
		return learningdomain.PolicyProposal{}, err
	}
	return p, nil
}

func (r *PostgresRepository) UpdateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error) {
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE policy_proposals SET status = $2, decided_by = $3, decided_at = $4, policy_id = $5
		WHERE id = $1
	`, p.ID, p.Status, p.DecidedBy, p.DecidedAt, p.PolicyID)
	if err != nil {
		return learningdomain.PolicyProposal{}, fmt.Errorf("update proposal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return learningdomain.PolicyProposal{}, ErrNotFound
	}
	return p, nil
}

// --- Scanner ---

const selectProposalSQL = `
	SELECT id, proposed_name, proposed_description, proposed_expression, proposed_effect,
	       proposed_action_type, proposed_priority,
	       pattern_summary, confidence, sample_size, time_window,
	       status, decided_by, decided_at, policy_id, created_at
	FROM policy_proposals`

type proposalScanRow interface {
	Scan(dest ...any) error
}

func scanProposal(row proposalScanRow) (learningdomain.PolicyProposal, error) {
	var p learningdomain.PolicyProposal
	if err := row.Scan(
		&p.ID, &p.ProposedName, &p.ProposedDescription, &p.ProposedExpression, &p.ProposedEffect,
		&p.ProposedActionType, &p.ProposedPriority,
		&p.PatternSummary, &p.Confidence, &p.SampleSize, &p.TimeWindow,
		&p.Status, &p.DecidedBy, &p.DecidedAt, &p.PolicyID, &p.CreatedAt,
	); err != nil {
		return learningdomain.PolicyProposal{}, fmt.Errorf("scan proposal: %w", err)
	}
	return p, nil
}
