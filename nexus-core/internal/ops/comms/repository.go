package comms

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"nexus-core/internal/ops/comms/repository/models"
	commsdomain "nexus-core/internal/ops/comms/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, in commsdomain.Draft) (commsdomain.Draft, error) {
	contentRaw, _ := json.Marshal(in.Content)
	row := models.Draft{
		ID:               uuid.New(),
		OrgID:            in.OrgID,
		IncidentID:       in.IncidentID,
		Channel:          in.Channel,
		Audience:         in.Audience,
		Status:           string(in.Status),
		ContentJSON:      datatypes.JSON(contentRaw),
		RequiresApproval: in.RequiresApproval,
		CreatedBy:        in.CreatedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return commsdomain.Draft{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) MarkStatus(ctx context.Context, orgID, draftID uuid.UUID, status commsdomain.Status) (commsdomain.Draft, error) {
	updates := map[string]any{"status": string(status)}
	if status == commsdomain.StatusSentInternal || status == commsdomain.StatusSentExternal {
		now := time.Now().UTC()
		updates["sent_at"] = now
	}
	if err := r.db.WithContext(ctx).Model(&models.Draft{}).
		Where("org_id = ? AND id = ?", orgID, draftID).
		Updates(updates).Error; err != nil {
		return commsdomain.Draft{}, err
	}

	var row models.Draft
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, draftID).Take(&row).Error; err != nil {
		return commsdomain.Draft{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) ListByIncident(ctx context.Context, orgID, incidentID uuid.UUID, limit int) ([]commsdomain.Draft, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	var rows []models.Draft
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND incident_id = ?", orgID, incidentID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]commsdomain.Draft, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(m models.Draft) commsdomain.Draft {
	var content map[string]any
	_ = json.Unmarshal(m.ContentJSON, &content)
	if content == nil {
		content = map[string]any{}
	}
	return commsdomain.Draft{
		ID:               m.ID,
		OrgID:            m.OrgID,
		IncidentID:       m.IncidentID,
		Channel:          m.Channel,
		Audience:         m.Audience,
		Status:           commsdomain.Status(m.Status),
		Content:          content,
		RequiresApproval: m.RequiresApproval,
		CreatedBy:        m.CreatedBy,
		CreatedAt:        m.CreatedAt,
		SentAt:           m.SentAt,
	}
}
