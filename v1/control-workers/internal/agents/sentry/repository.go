package sentry

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"control-workers/internal/agents/sentry/repository/models"
)

type SentryState struct {
	db *gorm.DB
}

func NewSentryState(db *gorm.DB) *SentryState {
	return &SentryState{db: db}
}

func (s *SentryState) GetBaseline(ctx context.Context, orgID uuid.UUID, toolName, metric string) (Baseline, error) {
	var row models.BaselineRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ? AND tool_name = ? AND metric = ?", orgID, toolName, metric).
		Take(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return Baseline{OrgID: orgID, ToolName: toolName, Metric: metric}, nil
		}
		return Baseline{}, err
	}
	return Baseline{
		OrgID:       row.OrgID,
		ToolName:    row.ToolName,
		Metric:      row.Metric,
		EWMA:        row.EWMAValue,
		SampleCount: row.SampleCount,
	}, nil
}

func (s *SentryState) UpsertBaseline(ctx context.Context, b Baseline) error {
	row := models.BaselineRow{
		OrgID:       b.OrgID,
		ToolName:    b.ToolName,
		Metric:      b.Metric,
		EWMAValue:   b.EWMA,
		SampleCount: b.SampleCount,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "org_id"}, {Name: "tool_name"}, {Name: "metric"}},
		DoUpdates: clause.Assignments(map[string]any{
			"ewma_value":   row.EWMAValue,
			"sample_count": row.SampleCount,
			"updated_at":   gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}

func (s *SentryState) GetFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string) (FingerprintState, error) {
	var row models.FingerprintRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ? AND fingerprint = ?", orgID, fingerprint).
		Take(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return FingerprintState{OrgID: orgID, Fingerprint: fingerprint}, nil
		}
		return FingerprintState{}, err
	}
	return FingerprintState{
		OrgID:       row.OrgID,
		Fingerprint: row.Fingerprint,
		IncidentID:  row.IncidentID,
		State:       row.State,
	}, nil
}

func (s *SentryState) UpsertFingerprint(ctx context.Context, f FingerprintState) error {
	row := models.FingerprintRow{
		OrgID:       f.OrgID,
		Fingerprint: f.Fingerprint,
		IncidentID:  f.IncidentID,
		State:       f.State,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "org_id"}, {Name: "fingerprint"}},
		DoUpdates: clause.Assignments(map[string]any{
			"incident_id": row.IncidentID,
			"state":       row.State,
			"updated_at":  gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}
