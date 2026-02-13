package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-gateway/internal/audit/repository/models"
	auditdomain "nexus-gateway/internal/audit/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, ev auditdomain.AuditEvent) error {
	inB, _ := json.Marshal(ev.InputRedacted)
	ctxB, _ := json.Marshal(ev.ContextRedacted)
	outB, _ := json.Marshal(ev.OutputRedacted)
	m := models.AuditEvent{
		ID:              uuid.New(),
		OrgID:           ev.OrgID,
		ToolID:          ev.ToolID,
		ToolName:        ev.ToolName,
		RequestID:       ev.RequestID,
		Actor:           ev.Actor,
		InputRedacted:   inB,
		ContextRedacted: ctxB,
		Decision:        string(ev.Decision),
		PolicyID:        ev.PolicyID,
		Reason:          ev.Reason,
		Status:          string(ev.Status),
		OutputRedacted:  outB,
		ErrorCode:       ev.ErrorCode,
		ErrorMessage:    ev.ErrorMessage,
		LatencyMS:       ev.LatencyMS,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *Repository) Query(ctx context.Context, orgID uuid.UUID, q auditdomain.Query) ([]auditdomain.AuditEvent, error) {
	tx := r.db.WithContext(ctx).Where("org_id = ?", orgID)
	if q.ToolName != nil && *q.ToolName != "" {
		tx = tx.Where("tool_name = ?", *q.ToolName)
	}
	if q.Decision != nil {
		tx = tx.Where("decision = ?", string(*q.Decision))
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", string(*q.Status))
	}
	if q.From != nil {
		tx = tx.Where("created_at >= ?", *q.From)
	}
	if q.To != nil {
		tx = tx.Where("created_at <= ?", *q.To)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var rows []models.AuditEvent
	if err := tx.Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]auditdomain.AuditEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(m models.AuditEvent) auditdomain.AuditEvent {
	var in any
	_ = json.Unmarshal(m.InputRedacted, &in)
	var ctx any
	_ = json.Unmarshal(m.ContextRedacted, &ctx)
	var out any
	if len(m.OutputRedacted) > 0 {
		_ = json.Unmarshal(m.OutputRedacted, &out)
	}
	return auditdomain.AuditEvent{
		ID:              m.ID,
		OrgID:           m.OrgID,
		ToolID:          m.ToolID,
		ToolName:        m.ToolName,
		RequestID:       m.RequestID,
		Actor:           m.Actor,
		InputRedacted:   in,
		ContextRedacted: ctx,
		Decision:        auditdomain.Decision(m.Decision),
		PolicyID:        m.PolicyID,
		Reason:          m.Reason,
		Status:          auditdomain.Status(m.Status),
		OutputRedacted:  out,
		ErrorCode:       m.ErrorCode,
		ErrorMessage:    m.ErrorMessage,
		LatencyMS:       m.LatencyMS,
		CreatedAt:       m.CreatedAt,
	}
}
