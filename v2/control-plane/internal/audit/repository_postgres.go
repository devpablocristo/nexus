package audit

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

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	auditdomain "nexus/v2/control-plane/internal/audit/usecases/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type PostgresRepository struct {
	db *sharedpostgres.DB
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, func(), error) {
	db, err := sharedpostgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open audit postgres database: %w", err)
	}
	if err := sharedpostgres.MigrateUp(ctx, db, "control-plane/audit", migrationFiles, "migrations"); err != nil {
		db.Close()
		return nil, nil, err
	}
	return &PostgresRepository{db: db}, db.Close, nil
}

func (r *PostgresRepository) Create(ctx context.Context, item auditdomain.AuditRecord) (auditdomain.AuditRecord, error) {
	now := nowUTC()
	id := uuid.New()
	if item.OccurredAt.IsZero() {
		item.OccurredAt = now
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	data := cloneData(item.Data)
	if data == nil {
		data = map[string]any{}
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return auditdomain.AuditRecord{}, fmt.Errorf("marshal audit data: %w", err)
	}

	var actorType any
	var actorID any
	if item.Actor != nil {
		actorType = item.Actor.Type
		actorID = item.Actor.ID
	}
	var actionID any
	if item.ActionID != "" {
		actionID = item.ActionID
	}
	var resourceID any
	if item.ResourceID != "" {
		resourceID = item.ResourceID
	}
	var resourceType any
	if item.ResourceType != "" {
		resourceType = item.ResourceType
	}

	_, err = r.db.Pool().Exec(ctx, `
		INSERT INTO audit_records (
			id, event_type, source_service, action_id, resource_id, resource_type,
			actor_type, actor_id, summary, data, occurred_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		id, item.EventType, item.SourceService, actionID, resourceID, resourceType,
		actorType, actorID, item.Summary, payload, item.OccurredAt, item.CreatedAt,
	)
	if err != nil {
		return auditdomain.AuditRecord{}, fmt.Errorf("insert audit record: %w", err)
	}

	item.ID = id.String()
	item.Data = data
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, filters ListFilters) ([]auditdomain.AuditRecord, error) {
	query := strings.Builder{}
	query.WriteString(`
		SELECT id, event_type, source_service, action_id, resource_id, resource_type,
		       actor_type, actor_id, summary, data, occurred_at, created_at
		FROM audit_records
		WHERE 1=1
	`)
	args := make([]any, 0, 8)

	addClause := func(clause string, value any) {
		args = append(args, value)
		_, _ = fmt.Fprintf(&query, " AND %s $%d", clause, len(args))
	}

	if filters.ActionID != "" {
		addClause("action_id =", filters.ActionID)
	}
	if filters.ResourceID != "" {
		addClause("resource_id =", filters.ResourceID)
	}
	if filters.ActorID != "" {
		addClause("actor_id =", filters.ActorID)
	}
	if filters.EventType != "" {
		addClause("event_type =", filters.EventType)
	}
	if !filters.From.IsZero() {
		addClause("occurred_at >=", filters.From)
	}
	if !filters.To.IsZero() {
		addClause("occurred_at <=", filters.To)
	}
	query.WriteString(" ORDER BY occurred_at DESC, created_at DESC")
	if filters.Limit > 0 {
		args = append(args, filters.Limit)
		_, _ = fmt.Fprintf(&query, " LIMIT $%d", len(args))
	}

	rows, err := r.db.Pool().Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	defer rows.Close()

	items := make([]auditdomain.AuditRecord, 0)
	for rows.Next() {
		item, err := scanAuditRecord(
			rows,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit records: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (auditdomain.AuditRecord, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, event_type, source_service, action_id, resource_id, resource_type,
		       actor_type, actor_id, summary, data, occurred_at, created_at
		FROM audit_records
		WHERE id = $1
	`, id)
	item, err := scanAuditRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auditdomain.AuditRecord{}, ErrNotFound
		}
		return auditdomain.AuditRecord{}, err
	}
	return item, nil
}

type scanRow interface {
	Scan(dest ...any) error
}

func scanAuditRecord(row scanRow) (auditdomain.AuditRecord, error) {
	var (
		id            uuid.UUID
		eventType     string
		sourceService string
		actionID      *string
		resourceID    *string
		resourceType  *string
		actorType     *string
		actorID       *string
		summary       string
		dataRaw       []byte
		occurredAt    time.Time
		createdAt     time.Time
	)
	if err := row.Scan(
		&id, &eventType, &sourceService, &actionID, &resourceID, &resourceType,
		&actorType, &actorID, &summary, &dataRaw, &occurredAt, &createdAt,
	); err != nil {
		return auditdomain.AuditRecord{}, fmt.Errorf("scan audit record: %w", err)
	}

	data := make(map[string]any)
	if len(dataRaw) > 0 {
		if err := json.Unmarshal(dataRaw, &data); err != nil {
			return auditdomain.AuditRecord{}, fmt.Errorf("decode audit data: %w", err)
		}
	}

	var actor *sharedaudit.Actor
	if actorType != nil || actorID != nil {
		actor = &sharedaudit.Actor{}
		if actorType != nil {
			actor.Type = *actorType
		}
		if actorID != nil {
			actor.ID = *actorID
		}
	}

	item := auditdomain.AuditRecord{
		ID:            id.String(),
		EventType:     eventType,
		SourceService: sourceService,
		Summary:       summary,
		Data:          data,
		OccurredAt:    occurredAt,
		CreatedAt:     createdAt,
		Actor:         actor,
	}
	if actionID != nil {
		item.ActionID = *actionID
	}
	if resourceID != nil {
		item.ResourceID = *resourceID
	}
	if resourceType != nil {
		item.ResourceType = *resourceType
	}
	return item, nil
}
