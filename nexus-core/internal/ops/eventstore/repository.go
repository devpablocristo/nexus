package eventstore

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nexus-core/internal/ops/eventstore/repository/models"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Append(ctx context.Context, event opsdomain.Envelope, schemaValid bool, validationError *string) (opsdomain.StoredEvent, error) {
	correlationRaw, _ := json.Marshal(event.Correlation)
	actorRaw, _ := json.Marshal(event.Actor)
	payloadRaw, _ := json.Marshal(event.Payload)

	row := models.Event{
		ID:              event.ID,
		EventType:       event.EventType,
		Version:         event.Version,
		OccurredAt:      event.OccurredAt.UTC(),
		OrgID:           event.OrgID,
		CorrelationJSON: datatypes.JSON(correlationRaw),
		ActorJSON:       datatypes.JSON(actorRaw),
		Source:          event.Source,
		PayloadJSON:     datatypes.JSON(payloadRaw),
		SchemaValid:     schemaValid,
		ValidationError: validationError,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return opsdomain.StoredEvent{}, err
	}
	return toDomainEvent(row), nil
}

func (r *Repository) ListAfterSequence(ctx context.Context, orgID uuid.UUID, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	var rows []models.Event
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND sequence > ?", orgID, afterSequence).
		Order("sequence ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]opsdomain.StoredEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainEvent(row))
	}
	return out, nil
}

func (r *Repository) GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error) {
	var row models.ConsumerOffset
	err := r.db.WithContext(ctx).Where("consumer_group = ?", consumerGroup).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return row.LastSeenSequence, nil
}

func (r *Repository) UpsertConsumerOffset(ctx context.Context, consumerGroup string, sequence int64) error {
	row := models.ConsumerOffset{
		ConsumerGroup:    consumerGroup,
		LastSeenSequence: sequence,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "consumer_group"}},
		DoUpdates: clause.Assignments(map[string]any{"last_seen_sequence": sequence, "updated_at": gorm.Expr("now()")}),
	}).Create(&row).Error
}

func (r *Repository) UpsertContract(ctx context.Context, in opsdomain.EventContract) error {
	schemaRaw, _ := json.Marshal(in.Schema)
	row := models.Contract{
		ID:         uuid.New(),
		EventType:  in.EventType,
		Version:    in.Version,
		SchemaJSON: datatypes.JSON(schemaRaw),
		Enabled:    in.Enabled,
		CreatedBy:  in.CreatedBy,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "event_type"}, {Name: "version"}},
		DoUpdates: clause.Assignments(map[string]any{
			"schema_json": row.SchemaJSON,
			"enabled":     row.Enabled,
		}),
	}).Create(&row).Error
}

func (r *Repository) GetContract(ctx context.Context, eventType string, version int) (opsdomain.EventContract, error) {
	var row models.Contract
	if err := r.db.WithContext(ctx).
		Where("event_type = ? AND version = ? AND enabled = true", eventType, version).
		Take(&row).Error; err != nil {
		return opsdomain.EventContract{}, err
	}
	var schema map[string]any
	_ = json.Unmarshal(row.SchemaJSON, &schema)
	if schema == nil {
		schema = map[string]any{}
	}
	return opsdomain.EventContract{
		EventType: row.EventType,
		Version:   row.Version,
		Schema:    schema,
		Enabled:   row.Enabled,
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
	}, nil
}

func toDomainEvent(m models.Event) opsdomain.StoredEvent {
	var correlation opsdomain.Correlation
	_ = json.Unmarshal(m.CorrelationJSON, &correlation)
	var actor opsdomain.Actor
	_ = json.Unmarshal(m.ActorJSON, &actor)
	var payload map[string]any
	_ = json.Unmarshal(m.PayloadJSON, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	return opsdomain.StoredEvent{
		Sequence: m.Sequence,
		Envelope: opsdomain.Envelope{
			ID:          m.ID,
			EventType:   m.EventType,
			Version:     m.Version,
			OccurredAt:  m.OccurredAt,
			OrgID:       m.OrgID,
			Correlation: correlation,
			Actor:       actor,
			Source:      m.Source,
			Payload:     payload,
		},
		SchemaValid:     m.SchemaValid,
		ValidationError: m.ValidationError,
		CreatedAt:       m.CreatedAt,
	}
}
