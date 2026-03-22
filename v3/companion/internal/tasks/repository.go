package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

var ErrNotFound = errors.New("task not found")

// Repository port de persistencia para tareas y entidades relacionadas.
type Repository interface {
	CreateTask(ctx context.Context, t domain.Task) (domain.Task, error)
	GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error)
	ListTasks(ctx context.Context, limit int) ([]domain.Task, error)
	UpdateTask(ctx context.Context, t domain.Task) (domain.Task, error)
	ListTasksByStatus(ctx context.Context, status string, limit int) ([]domain.Task, error)
	LatestProposeReviewRequestID(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error)

	InsertMessage(ctx context.Context, m domain.TaskMessage) (domain.TaskMessage, error)
	ListMessagesByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskMessage, error)

	InsertAction(ctx context.Context, a domain.TaskAction) (domain.TaskAction, error)
	UpdateActionReviewResult(ctx context.Context, actionID uuid.UUID, reviewRequestID *uuid.UUID, errMsg string) error
	ListActionsByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskAction, error)

	ListArtifactsByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskArtifact, error)
}

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(db *sharedpostgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const selectTask = `
	SELECT id, title, goal, status, priority, created_by, assigned_to, channel, summary,
	       context_json, created_at, updated_at, closed_at
	FROM companion_tasks`

func (r *PostgresRepository) CreateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	now := time.Now().UTC()
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = domain.TaskStatusNew
	}
	if t.Priority == "" {
		t.Priority = "normal"
	}
	if len(t.ContextJSON) == 0 {
		t.ContextJSON = json.RawMessage(`{}`)
	}
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO companion_tasks (
			id, title, goal, status, priority, created_by, assigned_to, channel, summary,
			context_json, created_at, updated_at, closed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, t.ID, t.Title, t.Goal, t.Status, t.Priority, t.CreatedBy, t.AssignedTo, t.Channel, t.Summary,
		t.ContextJSON, t.CreatedAt, t.UpdatedAt, t.ClosedAt)
	if err != nil {
		return domain.Task{}, fmt.Errorf("insert task: %w", err)
	}
	return t, nil
}

func (r *PostgresRepository) GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	row := r.db.Pool().QueryRow(ctx, selectTask+` WHERE id = $1`, id)
	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (r *PostgresRepository) ListTasks(ctx context.Context, limit int) ([]domain.Task, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool().Query(ctx, selectTask+` ORDER BY updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) UpdateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	t.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE companion_tasks SET
			title = $2, goal = $3, status = $4, priority = $5,
			created_by = $6, assigned_to = $7, channel = $8, summary = $9,
			context_json = $10, updated_at = $11, closed_at = $12
		WHERE id = $1
	`, t.ID, t.Title, t.Goal, t.Status, t.Priority, t.CreatedBy, t.AssignedTo, t.Channel, t.Summary,
		t.ContextJSON, t.UpdatedAt, t.ClosedAt)
	if err != nil {
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Task{}, ErrNotFound
	}
	return t, nil
}

func (r *PostgresRepository) ListTasksByStatus(ctx context.Context, status string, limit int) ([]domain.Task, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool().Query(ctx, selectTask+` WHERE status = $1 ORDER BY updated_at ASC LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list tasks by status: %w", err)
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) LatestProposeReviewRequestID(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error) {
	var rid uuid.UUID
	err := r.db.Pool().QueryRow(ctx, `
		SELECT review_request_id FROM companion_task_actions
		WHERE task_id = $1 AND action_type = $2 AND review_request_id IS NOT NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, taskID, TaskActionPropose).Scan(&rid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("latest propose review id: %w", err)
	}
	return rid, nil
}

func (r *PostgresRepository) InsertMessage(ctx context.Context, m domain.TaskMessage) (domain.TaskMessage, error) {
	now := time.Now().UTC()
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	m.CreatedAt = now
	if len(m.Metadata) == 0 {
		m.Metadata = json.RawMessage(`{}`)
	}
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO companion_task_messages (id, task_id, author_type, author_id, body, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, m.ID, m.TaskID, m.AuthorType, m.AuthorID, m.Body, m.Metadata, m.CreatedAt)
	if err != nil {
		return domain.TaskMessage{}, fmt.Errorf("insert message: %w", err)
	}
	return m, nil
}

func (r *PostgresRepository) ListMessagesByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskMessage, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, task_id, author_type, author_id, body, metadata, created_at
		FROM companion_task_messages WHERE task_id = $1 ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	var out []domain.TaskMessage
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) InsertAction(ctx context.Context, a domain.TaskAction) (domain.TaskAction, error) {
	now := time.Now().UTC()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = now
	if len(a.Payload) == 0 {
		a.Payload = json.RawMessage(`{}`)
	}
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO companion_task_actions (id, task_id, action_type, payload, review_request_id, error_message, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, a.ID, a.TaskID, a.ActionType, a.Payload, a.ReviewRequestID, nullIfEmpty(a.ErrorMessage), a.CreatedAt)
	if err != nil {
		return domain.TaskAction{}, fmt.Errorf("insert action: %w", err)
	}
	return a, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *PostgresRepository) UpdateActionReviewResult(ctx context.Context, actionID uuid.UUID, reviewRequestID *uuid.UUID, errMsg string) error {
	var rid any
	if reviewRequestID != nil {
		rid = *reviewRequestID
	}
	var em any
	if errMsg != "" {
		em = errMsg
	}
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE companion_task_actions SET review_request_id = $2, error_message = $3 WHERE id = $1
	`, actionID, rid, em)
	if err != nil {
		return fmt.Errorf("update action: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListActionsByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskAction, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, task_id, action_type, payload, review_request_id, error_message, created_at
		FROM companion_task_actions WHERE task_id = $1 ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list actions: %w", err)
	}
	defer rows.Close()
	var out []domain.TaskAction
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) ListArtifactsByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskArtifact, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, task_id, kind, uri, payload, created_at
		FROM companion_task_artifacts WHERE task_id = $1 ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	defer rows.Close()
	var out []domain.TaskArtifact
	for rows.Next() {
		var ar domain.TaskArtifact
		if err := rows.Scan(&ar.ID, &ar.TaskID, &ar.Kind, &ar.URI, &ar.Payload, &ar.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (domain.Task, error) {
	var t domain.Task
	var closed *time.Time
	err := row.Scan(
		&t.ID, &t.Title, &t.Goal, &t.Status, &t.Priority, &t.CreatedBy, &t.AssignedTo, &t.Channel, &t.Summary,
		&t.ContextJSON, &t.CreatedAt, &t.UpdatedAt, &closed,
	)
	if err != nil {
		return domain.Task{}, err
	}
	t.ClosedAt = closed
	return t, nil
}

func scanMessage(row rowScanner) (domain.TaskMessage, error) {
	var m domain.TaskMessage
	err := row.Scan(&m.ID, &m.TaskID, &m.AuthorType, &m.AuthorID, &m.Body, &m.Metadata, &m.CreatedAt)
	return m, err
}

func scanAction(row rowScanner) (domain.TaskAction, error) {
	var a domain.TaskAction
	var rid *uuid.UUID
	var errMsg *string
	err := row.Scan(&a.ID, &a.TaskID, &a.ActionType, &a.Payload, &rid, &errMsg, &a.CreatedAt)
	if err != nil {
		return domain.TaskAction{}, err
	}
	a.ReviewRequestID = rid
	if errMsg != nil {
		a.ErrorMessage = *errMsg
	}
	return a, nil
}
