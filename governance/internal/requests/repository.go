package requests

import (
	"context"
	"encoding/json"
	"errors"

	"fmt"
	"github.com/devpablocristo/core/errors/go/domainerr"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	requestdomain "github.com/devpablocristo/nexus/governance/internal/requests/usecases/domain"
)

// Sentinel errors
var (
	ErrNotFound             = domainerr.NotFound("not found")
	ErrInvalidState         = domainerr.Conflict("request is not in an executable state")
	ErrIdempotencyConflict  = domainerr.Conflict("idempotency key conflict")
)

// Repository define el port de persistencia para requests.
type Repository interface {
	Create(ctx context.Context, r requestdomain.Request) (requestdomain.Request, error)
	GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*requestdomain.Request, error)
	// List devuelve requests con filtro de tenancy aplicado en SQL para evitar
	// el bug de "post-filter después del LIMIT" (donde un usuario veía menos
	// resultados de los que le correspondían porque el LIMIT se llenaba con
	// rows de otros orgs antes del filtro).
	//   - allowAll=true: sin filtro (cross-org admin scope).
	//   - allowAll=false, orgID != nil: incluye sólo rows con org_id = *orgID.
	//   - allowAll=false, orgID == nil: incluye sólo rows con org_id IS NULL.
	List(ctx context.Context, status string, actionType string, limit int, orgID *string, allowAll bool) ([]requestdomain.Request, error)
	Update(ctx context.Context, r requestdomain.Request) (requestdomain.Request, error)
}

// IdempotencyStore define el port para el store de idempotencia.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (requestID uuid.UUID, response map[string]any, ok bool)
	Set(ctx context.Context, key string, requestID uuid.UUID, response map[string]any, expiresAt time.Time) error
}

type ResultReport struct {
	ID           uuid.UUID
	RequestID    uuid.UUID
	ResultKey    string
	ActorID      string
	OrgID        *string
	Success      bool
	Result       map[string]any
	ErrorMessage string
	DurationMs   int64
	PayloadHash  string
	CreatedAt    time.Time
}

type ResultReportStore interface {
	Get(ctx context.Context, requestID uuid.UUID, resultKey string) (ResultReport, bool, error)
	Save(ctx context.Context, report ResultReport) (ResultReport, error)
}

// --- Implementación PostgreSQL: Repository ---

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, req requestdomain.Request) (requestdomain.Request, error) {
	now := time.Now().UTC()
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	req.UpdatedAt = now

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO requests (
			id, org_id, idempotency_key, requester_type, requester_id, requester_name,
			action_type, target_system, target_resource, params, reason, context,
			risk_level, decision, decision_reason, policy_id,
			status, approval_id, execution_result, error_message,
			ai_summary, ai_degraded,
			evaluated_at, decided_at, executed_at, expires_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28)
	`,
		req.ID, req.OrgID, req.IdempotencyKey, req.RequesterType, req.RequesterID, req.RequesterName,
		req.ActionType, req.TargetSystem, req.TargetResource, req.Params, req.Reason, req.Context,
		req.RiskLevel, req.Decision, req.DecisionReason, req.PolicyID,
		req.Status, req.ApprovalID, req.ExecutionResult, req.ErrorMessage,
		req.AISummary, req.AIDegraded,
		req.EvaluatedAt, req.DecidedAt, req.ExecutedAt, req.ExpiresAt, req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) && req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
			return requestdomain.Request{}, ErrIdempotencyConflict
		}
		return requestdomain.Request{}, fmt.Errorf("insert request: %w", err)
	}
	return req, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error) {
	row := r.db.Pool().QueryRow(ctx, selectRequestSQL+` WHERE id = $1`, id)
	req, err := scanRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return requestdomain.Request{}, ErrNotFound
		}
		return requestdomain.Request{}, err
	}
	return req, nil
}

func (r *PostgresRepository) GetByIdempotencyKey(ctx context.Context, key string) (*requestdomain.Request, error) {
	row := r.db.Pool().QueryRow(ctx, selectRequestSQL+` WHERE idempotency_key = $1`, key)
	req, err := scanRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &req, nil
}

func (r *PostgresRepository) List(ctx context.Context, status, actionType string, limit int, orgID *string, allowAll bool) ([]requestdomain.Request, error) {
	query := selectRequestSQL + ` WHERE 1=1`
	args := []any{}
	argN := 1
	if status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argN)
		args = append(args, status)
		argN++
	}
	if actionType != "" {
		query += fmt.Sprintf(` AND action_type = $%d`, argN)
		args = append(args, actionType)
		argN++
	}
	if !allowAll {
		if orgID != nil {
			query += fmt.Sprintf(` AND org_id = $%d`, argN)
			args = append(args, *orgID)
			argN++
		} else {
			query += ` AND org_id IS NULL`
		}
	}
	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argN)
		args = append(args, limit)
	}

	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()

	out := make([]requestdomain.Request, 0)
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, req requestdomain.Request) (requestdomain.Request, error) {
	req.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE requests SET
			status = $2, risk_level = $3, decision = $4, decision_reason = $5,
			policy_id = $6, approval_id = $7, execution_result = $8, error_message = $9,
			ai_summary = $10, ai_degraded = $11,
			evaluated_at = $12, decided_at = $13, executed_at = $14, expires_at = $15, updated_at = $16
		WHERE id = $1
	`,
		req.ID, req.Status, req.RiskLevel, req.Decision, req.DecisionReason,
		req.PolicyID, req.ApprovalID, req.ExecutionResult, req.ErrorMessage,
		req.AISummary, req.AIDegraded,
		req.EvaluatedAt, req.DecidedAt, req.ExecutedAt, req.ExpiresAt, req.UpdatedAt,
	)
	if err != nil {
		return requestdomain.Request{}, fmt.Errorf("update request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return requestdomain.Request{}, ErrNotFound
	}
	return req, nil
}

// --- Implementación PostgreSQL: IdempotencyStore ---

type PostgresIdempotencyStore struct {
	db *sharedpostgres.DB
}

func NewPostgresIdempotencyStore(db *sharedpostgres.DB) *PostgresIdempotencyStore {
	return &PostgresIdempotencyStore{db: db}
}

func (s *PostgresIdempotencyStore) Get(ctx context.Context, key string) (requestID uuid.UUID, response map[string]any, ok bool) {
	row := s.db.Pool().QueryRow(ctx, `
		SELECT request_id, response FROM idempotency_keys
		WHERE key = $1 AND expires_at > now()
	`, key)
	var respJSON []byte
	if err := row.Scan(&requestID, &respJSON); err != nil {
		return uuid.Nil, nil, false
	}
	if err := json.Unmarshal(respJSON, &response); err != nil {
		return uuid.Nil, nil, false
	}
	return requestID, response, true
}

func (s *PostgresIdempotencyStore) Set(ctx context.Context, key string, requestID uuid.UUID, response map[string]any, expiresAt time.Time) error {
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(24 * time.Hour)
	}
	respJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal idempotency response: %w", err)
	}
	_, err = s.db.Pool().Exec(ctx, `
		INSERT INTO idempotency_keys (key, request_id, response, created_at, expires_at)
		VALUES ($1, $2, $3, now(), $4)
		ON CONFLICT (key) DO UPDATE SET response = $3, expires_at = $4
	`, key, requestID, respJSON, expiresAt)
	if err != nil {
		return fmt.Errorf("set idempotency key: %w", err)
	}
	return nil
}

type PostgresResultReportStore struct {
	db *sharedpostgres.DB
}

func NewPostgresResultReportStore(db *sharedpostgres.DB) *PostgresResultReportStore {
	return &PostgresResultReportStore{db: db}
}

func (s *PostgresResultReportStore) Get(ctx context.Context, requestID uuid.UUID, resultKey string) (ResultReport, bool, error) {
	row := s.db.Pool().QueryRow(ctx, `
		SELECT id, request_id, result_key, actor_id, org_id, success, result_json, error_message, duration_ms, payload_hash, created_at
		FROM request_result_reports
		WHERE request_id = $1 AND result_key = $2
	`, requestID, resultKey)
	report, err := scanResultReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResultReport{}, false, nil
		}
		return ResultReport{}, false, err
	}
	return report, true, nil
}

func (s *PostgresResultReportStore) Save(ctx context.Context, report ResultReport) (ResultReport, error) {
	if report.ID == uuid.Nil {
		report.ID = uuid.New()
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now().UTC()
	}
	resultJSON, err := json.Marshal(report.Result)
	if err != nil {
		return ResultReport{}, fmt.Errorf("marshal result report payload: %w", err)
	}
	row := s.db.Pool().QueryRow(ctx, `
		INSERT INTO request_result_reports
			(id, request_id, result_key, actor_id, org_id, success, result_json, error_message, duration_ms, payload_hash, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (request_id, result_key) DO NOTHING
		RETURNING id, request_id, result_key, actor_id, org_id, success, result_json, error_message, duration_ms, payload_hash, created_at
	`, report.ID, report.RequestID, report.ResultKey, report.ActorID, report.OrgID, report.Success, resultJSON, report.ErrorMessage, report.DurationMs, report.PayloadHash, report.CreatedAt)
	saved, err := scanResultReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResultReport{}, ErrIdempotencyConflict
		}
		return ResultReport{}, fmt.Errorf("insert result report: %w", err)
	}
	return saved, nil
}

// --- Scanner ---

const selectRequestSQL = `
	SELECT id, org_id, idempotency_key, requester_type, requester_id, requester_name,
	       action_type, target_system, target_resource, params, reason, context,
	       risk_level, decision, decision_reason, policy_id,
	       status, approval_id, execution_result, error_message,
	       ai_summary, ai_degraded,
	       evaluated_at, decided_at, executed_at, expires_at, created_at, updated_at
	FROM requests`

type requestScanRow interface {
	Scan(dest ...any) error
}

func scanRequest(row requestScanRow) (requestdomain.Request, error) {
	var req requestdomain.Request
	if err := row.Scan(
		&req.ID, &req.OrgID, &req.IdempotencyKey, &req.RequesterType, &req.RequesterID, &req.RequesterName,
		&req.ActionType, &req.TargetSystem, &req.TargetResource, &req.Params, &req.Reason, &req.Context,
		&req.RiskLevel, &req.Decision, &req.DecisionReason, &req.PolicyID,
		&req.Status, &req.ApprovalID, &req.ExecutionResult, &req.ErrorMessage,
		&req.AISummary, &req.AIDegraded,
		&req.EvaluatedAt, &req.DecidedAt, &req.ExecutedAt, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	); err != nil {
		return requestdomain.Request{}, fmt.Errorf("scan request: %w", err)
	}
	return req, nil
}

func scanResultReport(row requestScanRow) (ResultReport, error) {
	var report ResultReport
	var resultJSON []byte
	if err := row.Scan(
		&report.ID, &report.RequestID, &report.ResultKey, &report.ActorID, &report.OrgID,
		&report.Success, &resultJSON, &report.ErrorMessage, &report.DurationMs,
		&report.PayloadHash, &report.CreatedAt,
	); err != nil {
		return ResultReport{}, err
	}
	if len(resultJSON) > 0 {
		if err := json.Unmarshal(resultJSON, &report.Result); err != nil {
			return ResultReport{}, fmt.Errorf("decode result report payload: %w", err)
		}
	}
	if report.Result == nil {
		report.Result = make(map[string]any)
	}
	return report, nil
}
