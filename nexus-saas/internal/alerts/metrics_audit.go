package alerts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditMetricsSource derives alert metric values from the audit_events table.
type AuditMetricsSource struct {
	db *gorm.DB
}

func NewAuditMetricsSource(db *gorm.DB) *AuditMetricsSource {
	return &AuditMetricsSource{db: db}
}

func (s *AuditMetricsSource) DenyRate(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int) (float64, error) {
	return s.decisionRate(ctx, orgID, toolName, windowSeconds, "deny")
}

func (s *AuditMetricsSource) ErrorRate(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int) (float64, error) {
	return s.statusRate(ctx, orgID, toolName, windowSeconds, "error")
}

func (s *AuditMetricsSource) RateLimitedCount(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int) (float64, error) {
	since := time.Now().Add(-time.Duration(windowSeconds) * time.Second)
	q := s.db.WithContext(ctx).Table("audit_events").
		Where("org_id = ? AND created_at >= ?", orgID, since).
		Where("error_code = 'RATE_LIMITED'")
	if toolName != nil && *toolName != "" {
		q = q.Where("tool_name = ?", *toolName)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return float64(count), nil
}

func (s *AuditMetricsSource) decisionRate(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int, decision string) (float64, error) {
	since := time.Now().Add(-time.Duration(windowSeconds) * time.Second)
	base := s.db.WithContext(ctx).Table("audit_events").
		Where("org_id = ? AND created_at >= ?", orgID, since)
	if toolName != nil && *toolName != "" {
		base = base.Where("tool_name = ?", *toolName)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}

	var matched int64
	if err := base.Where("decision = ?", decision).Count(&matched).Error; err != nil {
		return 0, err
	}
	return float64(matched) / float64(total), nil
}

func (s *AuditMetricsSource) statusRate(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int, status string) (float64, error) {
	since := time.Now().Add(-time.Duration(windowSeconds) * time.Second)
	base := s.db.WithContext(ctx).Table("audit_events").
		Where("org_id = ? AND created_at >= ?", orgID, since)
	if toolName != nil && *toolName != "" {
		base = base.Where("tool_name = ?", *toolName)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}

	var matched int64
	if err := base.Where("status = ?", status).Count(&matched).Error; err != nil {
		return 0, err
	}
	return float64(matched) / float64(total), nil
}

var _ MetricsSource = (*AuditMetricsSource)(nil)
