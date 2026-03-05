package usagemetering

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	saasmetrics "nexus-saas/internal/shared/metrics"
)

const (
	CounterAPICalls        = "api_calls"
	CounterEventsIngested  = "events_ingested"
	CounterIncidentsOpened = "incidents_opened"
	CounterActionsExecuted = "actions_executed"
)

// MeteringPort is the narrow interface consumed by integration points.
// Each consumer package declares its own copy (hexagonal pattern).
type MeteringPort interface {
	Increment(ctx context.Context, orgID uuid.UUID, counter string) error
}

type usageRow struct {
	OrgID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Period    time.Time `gorm:"type:date;primaryKey"`
	Counter   string    `gorm:"primaryKey"`
	Value     int64
	UpdatedAt time.Time
}

func (usageRow) TableName() string { return "org_usage_counters" }

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Increment atomically increments a counter for the current billing period.
// Errors are non-fatal — callers should always discard them.
func (r *Repository) Increment(ctx context.Context, orgID uuid.UUID, counter string) error {
	now := time.Now().UTC()
	period := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	err := r.db.WithContext(ctx).Exec(
		`INSERT INTO org_usage_counters (org_id, period, counter, value, updated_at)
		 VALUES (?, ?, ?, 1, now())
		 ON CONFLICT (org_id, period, counter)
		 DO UPDATE SET value = org_usage_counters.value + 1, updated_at = now()`,
		orgID, period, counter,
	).Error
	if err == nil {
		saasmetrics.UsageMeteringEvents.WithLabelValues(orgID.String(), counter).Inc()
	}
	return err
}

// IncrementEvent performs idempotent usage ingestion keyed by eventID.
func (r *Repository) IncrementEvent(ctx context.Context, eventID string, orgID uuid.UUID, counter string) error {
	if eventID == "" {
		return r.Increment(ctx, orgID, counter)
	}
	now := time.Now().UTC()
	period := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	incremented := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(
			`INSERT INTO saas_usage_event_dedup (event_id, org_id, counter, created_at)
			 VALUES (?, ?, ?, now())
			 ON CONFLICT (event_id) DO NOTHING`,
			eventID, orgID, counter,
		)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := tx.Exec(
			`INSERT INTO org_usage_counters (org_id, period, counter, value, updated_at)
			 VALUES (?, ?, ?, 1, now())
			 ON CONFLICT (org_id, period, counter)
			 DO UPDATE SET value = org_usage_counters.value + 1, updated_at = now()`,
			orgID, period, counter,
		).Error; err != nil {
			return err
		}
		incremented = true
		return nil
	})
	if err == nil && incremented {
		saasmetrics.UsageMeteringEvents.WithLabelValues(orgID.String(), counter).Inc()
	}
	return err
}
