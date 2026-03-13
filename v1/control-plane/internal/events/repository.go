package events

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"control-plane/internal/events/repository/models"
	eventdomain "control-plane/internal/events/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, ev eventdomain.Event) (eventdomain.Event, error) {
	payload, _ := json.Marshal(ev.Payload)
	row := models.Event{
		OrgID:     ev.OrgID,
		EventType: ev.EventType,
		Payload:   payload,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return eventdomain.Event{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) ListByCursor(ctx context.Context, orgID uuid.UUID, cursor int64, limit int) ([]eventdomain.Event, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	q := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("id asc").
		Limit(limit)
	if cursor > 0 {
		q = q.Where("id > ?", cursor)
	}
	var rows []models.Event
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]eventdomain.Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) ListRecent(ctx context.Context, orgID uuid.UUID, limit int) ([]eventdomain.Event, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var rows []models.Event
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("id desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]eventdomain.Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(m models.Event) eventdomain.Event {
	var payload map[string]any
	_ = json.Unmarshal(m.Payload, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	return eventdomain.Event{
		ID:        m.ID,
		OrgID:     m.OrgID,
		EventType: m.EventType,
		Payload:   payload,
		CreatedAt: m.CreatedAt,
	}
}
