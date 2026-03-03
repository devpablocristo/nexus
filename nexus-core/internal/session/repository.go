package session

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nexus-core/internal/session/repository/models"
	domain "nexus-core/internal/session/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetOrCreate(ctx context.Context, orgID uuid.UUID, sessionID string, actor *string) (domain.AgentSession, error) {
	metaJSON, _ := json.Marshal(map[string]any{})
	row := models.AgentSession{
		OrgID:     orgID,
		SessionID: sessionID,
		Actor:     actor,
		Metadata:  metaJSON,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "org_id"}, {Name: "session_id"}},
			DoNothing: true,
		}).
		Create(&row).Error
	if err != nil {
		return domain.AgentSession{}, err
	}
	var existing models.AgentSession
	if err := r.db.WithContext(ctx).Where("org_id = ? AND session_id = ?", orgID, sessionID).First(&existing).Error; err != nil {
		return domain.AgentSession{}, err
	}
	return toDomain(existing), nil
}

func (r *Repository) IncrementCall(ctx context.Context, orgID uuid.UUID, sessionID string, isWrite bool, isDenial bool) error {
	updates := map[string]any{
		"total_calls": gorm.Expr("total_calls + 1"),
		"last_call_at": time.Now(),
	}
	if isWrite {
		updates["total_writes"] = gorm.Expr("total_writes + 1")
	}
	if isDenial {
		updates["total_denials"] = gorm.Expr("total_denials + 1")
	}
	return r.db.WithContext(ctx).
		Model(&models.AgentSession{}).
		Where("org_id = ? AND session_id = ?", orgID, sessionID).
		Updates(updates).Error
}

func (r *Repository) GetBySessionID(ctx context.Context, orgID uuid.UUID, sessionID string) (domain.AgentSession, error) {
	var row models.AgentSession
	if err := r.db.WithContext(ctx).Where("org_id = ? AND session_id = ?", orgID, sessionID).First(&row).Error; err != nil {
		return domain.AgentSession{}, err
	}
	return toDomain(row), nil
}

func toDomain(m models.AgentSession) domain.AgentSession {
	var meta map[string]any
	_ = json.Unmarshal(m.Metadata, &meta)
	if meta == nil {
		meta = map[string]any{}
	}
	return domain.AgentSession{
		ID:           m.ID,
		OrgID:        m.OrgID,
		SessionID:    m.SessionID,
		Actor:        m.Actor,
		TotalCalls:   m.TotalCalls,
		TotalWrites:  m.TotalWrites,
		TotalDenials: m.TotalDenials,
		Metadata:     meta,
		CreatedAt:    m.CreatedAt,
		LastCallAt:   m.LastCallAt,
	}
}
