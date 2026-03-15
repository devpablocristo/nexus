package action

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, func(), error) {
	db, err := sharedpostgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open actions postgres database: %w", err)
	}
	repo, err := NewPostgresRepositoryWithDB(ctx, db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return repo, db.Close, nil
}

func NewPostgresRepositoryWithDB(ctx context.Context, db *sharedpostgres.DB) (*PostgresRepository, error) {
	if err := sharedpostgres.MigrateUp(ctx, db, "data-plane/actions", migrationFiles, "migrations"); err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, item actiondomain.Action) (actiondomain.Action, error) {
	now := time.Now().UTC()
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	if err := r.insertOrUpdate(ctx, nil, item, true); err != nil {
		return actiondomain.Action{}, err
	}
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]actiondomain.Action, error) {
	query := `
		SELECT id, type, status, decision, resource_id, resource_type, source_system, justification,
		       requested_by, proposed_by, payload, metadata, risk, evidence, approval, lease, execution,
		       expires_at, created_at, updated_at
		FROM actions
		WHERE ($1 = '' OR type = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC, id DESC
	`
	args := []any{filters.ActionType, filters.Status}
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, filters.Limit)
	}

	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list actions: %w", err)
	}
	defer rows.Close()

	items := make([]actiondomain.Action, 0)
	for rows.Next() {
		item, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate actions: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (actiondomain.Action, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, type, status, decision, resource_id, resource_type, source_system, justification,
		       requested_by, proposed_by, payload, metadata, risk, evidence, approval, lease, execution,
		       expires_at, created_at, updated_at
		FROM actions
		WHERE id = $1
	`, id)
	item, err := scanAction(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actiondomain.Action{}, ErrNotFound
		}
		return actiondomain.Action{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Decide(ctx context.Context, id uuid.UUID, status actiondomain.ApprovalStatus, decidedBy actiondomain.ActorRef, comment string, decidedAt time.Time) (actiondomain.Action, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return actiondomain.Action{}, fmt.Errorf("begin decide tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, err := r.getForUpdate(ctx, tx, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if item.Approval == nil || item.Approval.Status != actiondomain.ApprovalStatusPending {
		return actiondomain.Action{}, ErrApprovalNotPending
	}

	item.Approval.Status = status
	item.Approval.DecidedBy = &decidedBy
	item.Approval.Comment = comment
	item.Approval.DecidedAt = &decidedAt
	item.Approval.UpdatedAt = decidedAt
	item.UpdatedAt = decidedAt
	switch status {
	case actiondomain.ApprovalStatusApproved:
		item.Approval.GrantedCount = item.Approval.RequiredCount
		item.Status = actiondomain.ActionStatusApproved
		item.Decision = actiondomain.DecisionAllow
	case actiondomain.ApprovalStatusRejected:
		item.Status = actiondomain.ActionStatusRejected
		item.Decision = actiondomain.DecisionDeny
	}

	if err := r.insertOrUpdate(ctx, tx, item, false); err != nil {
		return actiondomain.Action{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return actiondomain.Action{}, fmt.Errorf("commit decide tx: %w", err)
	}
	return item, nil
}

func (r *PostgresRepository) IssueLease(ctx context.Context, id uuid.UUID, lease actiondomain.ExecutionLease) (actiondomain.Action, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return actiondomain.Action{}, fmt.Errorf("begin issue lease tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, err := r.getForUpdate(ctx, tx, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if item.Lease != nil {
		switch item.Lease.Status {
		case actiondomain.LeaseStatusActive, actiondomain.LeaseStatusUsed:
			return actiondomain.Action{}, ErrLeaseAlreadyIssued
		}
	}

	item.Lease = &lease
	item.Status = actiondomain.ActionStatusLeased
	item.Decision = actiondomain.DecisionAllow
	item.UpdatedAt = lease.CreatedAt

	if err := r.insertOrUpdate(ctx, tx, item, false); err != nil {
		return actiondomain.Action{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return actiondomain.Action{}, fmt.Errorf("commit issue lease tx: %w", err)
	}
	return item, nil
}

func (r *PostgresRepository) ConsumeLeaseAndMarkExecuted(ctx context.Context, id, leaseID uuid.UUID, execution actiondomain.ExecutionResult) (actiondomain.Action, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return actiondomain.Action{}, fmt.Errorf("begin execute tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, err := r.getForUpdate(ctx, tx, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if item.Execution != nil {
		return actiondomain.Action{}, ErrActionAlreadyExecuted
	}
	if item.Lease == nil {
		return actiondomain.Action{}, ErrLeaseNotFound
	}
	if item.Lease.ID != leaseID {
		return actiondomain.Action{}, ErrLeaseMismatch
	}
	if item.Lease.Status != actiondomain.LeaseStatusActive {
		return actiondomain.Action{}, ErrLeaseNotActive
	}
	if !item.Lease.ExpiresAt.IsZero() && execution.ExecutedAt.After(item.Lease.ExpiresAt) {
		item.Lease.Status = actiondomain.LeaseStatusExpired
		item.Status = actiondomain.ActionStatusApproved
		item.UpdatedAt = execution.ExecutedAt
		if err := r.insertOrUpdate(ctx, tx, item, false); err != nil {
			return actiondomain.Action{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return actiondomain.Action{}, fmt.Errorf("commit expired lease tx: %w", err)
		}
		return actiondomain.Action{}, ErrLeaseExpired
	}

	item.Lease.Status = actiondomain.LeaseStatusUsed
	item.Lease.UsedAt = &execution.ExecutedAt
	item.Execution = &execution
	item.Status = actiondomain.ActionStatusExecuted
	item.Decision = actiondomain.DecisionAllow
	item.UpdatedAt = execution.ExecutedAt

	if err := r.insertOrUpdate(ctx, tx, item, false); err != nil {
		return actiondomain.Action{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return actiondomain.Action{}, fmt.Errorf("commit execute tx: %w", err)
	}
	return item, nil
}

type actionQueryable interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *PostgresRepository) getForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (actiondomain.Action, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, type, status, decision, resource_id, resource_type, source_system, justification,
		       requested_by, proposed_by, payload, metadata, risk, evidence, approval, lease, execution,
		       expires_at, created_at, updated_at
		FROM actions
		WHERE id = $1
		FOR UPDATE
	`, id)
	item, err := scanAction(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actiondomain.Action{}, ErrNotFound
		}
		return actiondomain.Action{}, err
	}
	return item, nil
}

func (r *PostgresRepository) insertOrUpdate(ctx context.Context, q actionQueryable, item actiondomain.Action, insert bool) error {
	if q == nil {
		q = r.db.Pool()
	}
	requestedBy, err := json.Marshal(item.RequestedBy)
	if err != nil {
		return fmt.Errorf("marshal action requested_by: %w", err)
	}
	proposedBy, err := json.Marshal(item.ProposedBy)
	if err != nil {
		return fmt.Errorf("marshal action proposed_by: %w", err)
	}
	payload, err := marshalRawJSON(item.Payload)
	if err != nil {
		return fmt.Errorf("marshal action payload: %w", err)
	}
	metadata, err := marshalJSONMap(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal action metadata: %w", err)
	}
	risk, err := json.Marshal(item.Risk)
	if err != nil {
		return fmt.Errorf("marshal action risk: %w", err)
	}
	evidence, err := json.Marshal(item.Evidence)
	if err != nil {
		return fmt.Errorf("marshal action evidence: %w", err)
	}
	approval, err := marshalOptionalJSON(item.Approval)
	if err != nil {
		return fmt.Errorf("marshal action approval: %w", err)
	}
	lease, err := marshalOptionalJSON(item.Lease)
	if err != nil {
		return fmt.Errorf("marshal action lease: %w", err)
	}
	execution, err := marshalOptionalJSON(item.Execution)
	if err != nil {
		return fmt.Errorf("marshal action execution: %w", err)
	}

	sql := `
		UPDATE actions
		SET type = $2,
			status = $3,
			decision = $4,
			resource_id = $5,
			resource_type = $6,
			source_system = $7,
			justification = $8,
			requested_by = $9,
			proposed_by = $10,
			payload = $11,
			metadata = $12,
			risk = $13,
			evidence = $14,
			approval = $15,
			lease = $16,
			execution = $17,
			expires_at = $18,
			created_at = $19,
			updated_at = $20
		WHERE id = $1
	`
	if insert {
		sql = `
			INSERT INTO actions (
				id, type, status, decision, resource_id, resource_type, source_system, justification,
				requested_by, proposed_by, payload, metadata, risk, evidence, approval, lease, execution,
				expires_at, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		`
	}
	if _, err := q.Exec(ctx, sql,
		item.ID, item.Type, item.Status, item.Decision, item.ResourceID, item.ResourceType, item.SourceSystem, item.Justification,
		requestedBy, proposedBy, payload, metadata, risk, evidence, approval, lease, execution,
		nullableTime(item.ExpiresAt), item.CreatedAt, item.UpdatedAt,
	); err != nil {
		return fmt.Errorf("persist action: %w", err)
	}
	return nil
}

type actionScanRow interface {
	Scan(dest ...any) error
}

func scanAction(row actionScanRow) (actiondomain.Action, error) {
	var (
		item        actiondomain.Action
		requestedBy []byte
		proposedBy  []byte
		payload     []byte
		metadata    []byte
		risk        []byte
		evidence    []byte
		approval    []byte
		lease       []byte
		execution   []byte
		expiresAt   *time.Time
	)
	if err := row.Scan(
		&item.ID, &item.Type, &item.Status, &item.Decision, &item.ResourceID, &item.ResourceType, &item.SourceSystem, &item.Justification,
		&requestedBy, &proposedBy, &payload, &metadata, &risk, &evidence, &approval, &lease, &execution,
		&expiresAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return actiondomain.Action{}, fmt.Errorf("scan action: %w", err)
	}
	if expiresAt != nil {
		item.ExpiresAt = *expiresAt
	}
	if err := json.Unmarshal(requestedBy, &item.RequestedBy); err != nil {
		return actiondomain.Action{}, fmt.Errorf("decode action requested_by: %w", err)
	}
	if err := json.Unmarshal(proposedBy, &item.ProposedBy); err != nil {
		return actiondomain.Action{}, fmt.Errorf("decode action proposed_by: %w", err)
	}
	item.Payload = append(json.RawMessage(nil), payload...)
	if err := unmarshalJSONMap(metadata, &item.Metadata); err != nil {
		return actiondomain.Action{}, fmt.Errorf("decode action metadata: %w", err)
	}
	if err := json.Unmarshal(risk, &item.Risk); err != nil {
		return actiondomain.Action{}, fmt.Errorf("decode action risk: %w", err)
	}
	if err := json.Unmarshal(evidence, &item.Evidence); err != nil {
		return actiondomain.Action{}, fmt.Errorf("decode action evidence: %w", err)
	}
	if len(approval) > 0 {
		item.Approval = &actiondomain.Approval{}
		if err := json.Unmarshal(approval, item.Approval); err != nil {
			return actiondomain.Action{}, fmt.Errorf("decode action approval: %w", err)
		}
	}
	if len(lease) > 0 {
		item.Lease = &actiondomain.ExecutionLease{}
		if err := json.Unmarshal(lease, item.Lease); err != nil {
			return actiondomain.Action{}, fmt.Errorf("decode action lease: %w", err)
		}
	}
	if len(execution) > 0 {
		item.Execution = &actiondomain.ExecutionResult{}
		if err := json.Unmarshal(execution, item.Execution); err != nil {
			return actiondomain.Action{}, fmt.Errorf("decode action execution: %w", err)
		}
	}
	return item, nil
}

func marshalRawJSON(value json.RawMessage) ([]byte, error) {
	if len(value) == 0 {
		return []byte(`{}`), nil
	}
	var payload any
	if err := json.Unmarshal(value, &payload); err != nil {
		return nil, err
	}
	return json.Marshal(payload)
}

func marshalJSONMap(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte(`{}`), nil
	}
	return json.Marshal(value)
}

func marshalOptionalJSON[T any](value *T) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func unmarshalJSONMap(raw []byte, out *map[string]any) error {
	if len(raw) == 0 {
		*out = map[string]any{}
		return nil
	}
	return json.Unmarshal(raw, out)
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
