package incidents

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-saas/internal/incidents/repository/models"
	incidentdomain "nexus-saas/internal/incidents/usecases/domain"
	"nexus/pkg/types"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, in incidentdomain.Incident) (incidentdomain.Incident, error) {
	related, _ := json.Marshal(in.RelatedActionIDs)
	evidence, _ := json.Marshal(in.EvidenceRefs)
	row := models.Incident{
		ID:               uuid.New(),
		OrgID:            in.OrgID,
		Severity:         string(in.Severity),
		Status:           string(in.Status),
		Title:            in.Title,
		Summary:          in.Summary,
		RelatedActionIDs: related,
		EvidenceRefs:     evidence,
		CreatedBy:        in.CreatedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return incidentdomain.Incident{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]incidentdomain.Incident, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	q := r.db.WithContext(ctx).Where("org_id = ?", orgID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.Incident
	if err := q.Order("opened_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]incidentdomain.Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (incidentdomain.Incident, error) {
	var row models.Incident
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return incidentdomain.Incident{}, types.NewHTTPError(404, types.ErrCodeNotFound, "incident not found")
		}
		return incidentdomain.Incident{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Close(ctx context.Context, orgID, id uuid.UUID) (incidentdomain.Incident, error) {
	if err := r.db.WithContext(ctx).Model(&models.Incident{}).
		Where("org_id = ? AND id = ?", orgID, id).
		Updates(map[string]any{"status": string(incidentdomain.StatusClosed), "closed_at": gorm.Expr("now()")}).Error; err != nil {
		return incidentdomain.Incident{}, err
	}
	return r.GetByID(ctx, orgID, id)
}

func toDomain(m models.Incident) incidentdomain.Incident {
	var related []string
	_ = json.Unmarshal(m.RelatedActionIDs, &related)
	var evidence []string
	_ = json.Unmarshal(m.EvidenceRefs, &evidence)
	return incidentdomain.Incident{
		ID:               m.ID,
		OrgID:            m.OrgID,
		Severity:         incidentdomain.Severity(m.Severity),
		Status:           incidentdomain.Status(m.Status),
		Title:            m.Title,
		Summary:          m.Summary,
		RelatedActionIDs: related,
		EvidenceRefs:     evidence,
		CreatedBy:        m.CreatedBy,
		OpenedAt:         m.OpenedAt,
		ClosedAt:         m.ClosedAt,
	}
}
