package diagnosis

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"nexus-core/internal/ops/diagnosis/repository/models"
	diagnosisdomain "nexus-core/internal/ops/diagnosis/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error) {
	reportRaw, _ := json.Marshal(in.Report)
	row := models.Report{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		IncidentID:      in.IncidentID,
		Provider:        in.Provider,
		Model:           in.Model,
		Status:          string(in.Status),
		ReportJSON:      datatypes.JSON(reportRaw),
		ValidationError: in.ValidationError,
		CreatedBy:       in.CreatedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return diagnosisdomain.Report{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) ListByIncident(ctx context.Context, orgID uuid.UUID, incidentID uuid.UUID, limit int) ([]diagnosisdomain.Report, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	var rows []models.Report
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND incident_id = ?", orgID, incidentID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]diagnosisdomain.Report, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(m models.Report) diagnosisdomain.Report {
	var report map[string]any
	_ = json.Unmarshal(m.ReportJSON, &report)
	if report == nil {
		report = map[string]any{}
	}
	return diagnosisdomain.Report{
		ID:              m.ID,
		OrgID:           m.OrgID,
		IncidentID:      m.IncidentID,
		Provider:        m.Provider,
		Model:           m.Model,
		Status:          diagnosisdomain.Status(m.Status),
		Report:          report,
		ValidationError: m.ValidationError,
		CreatedBy:       m.CreatedBy,
		CreatedAt:       m.CreatedAt,
	}
}
