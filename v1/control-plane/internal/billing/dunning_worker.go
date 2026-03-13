package billing

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	billingdomain "control-plane/internal/billing/usecases/domain"
)

const defaultGracePeriod = 14 * 24 * time.Hour

type dunningRepo interface {
	FindPastDueBefore(ctx context.Context, cutoff time.Time) ([]billingdomain.TenantBilling, error)
}

type dunningAdminPort interface {
	AutoSuspend(ctx context.Context, targetOrgID uuid.UUID) error
}

// DunningWorker auto-suspends tenants that remain past_due after the grace period.
type DunningWorker struct {
	repo        dunningRepo
	adminUC     dunningAdminPort
	gracePeriod time.Duration
	logger      zerolog.Logger
}

func NewDunningWorker(repo dunningRepo, adminUC dunningAdminPort, logger zerolog.Logger) *DunningWorker {
	return &DunningWorker{
		repo:        repo,
		adminUC:     adminUC,
		gracePeriod: defaultGracePeriod,
		logger:      logger,
	}
}

func (w *DunningWorker) RunOnce(ctx context.Context) {
	if w == nil || w.repo == nil || w.adminUC == nil {
		return
	}
	cutoff := time.Now().UTC().Add(-w.gracePeriod)
	tenants, err := w.repo.FindPastDueBefore(ctx, cutoff)
	if err != nil {
		w.logger.Error().Err(err).Msg("dunning: failed to query past_due tenants")
		return
	}
	for _, t := range tenants {
		if err := w.adminUC.AutoSuspend(ctx, t.OrgID); err != nil {
			w.logger.Error().Err(err).Str("org_id", t.OrgID.String()).Msg("dunning: auto-suspend failed")
			continue
		}
		w.logger.Info().Str("org_id", t.OrgID.String()).Msg("dunning: tenant auto-suspended after grace period")
	}
}
