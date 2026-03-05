package billing

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-saas/internal/billing/repository/models"
	billingdomain "nexus-saas/internal/billing/usecases/domain"
	"nexus-saas/internal/usagemetering"
	"nexus/pkg/types"
)

type Repository struct {
	db *gorm.DB
}

type stripeFieldsUpdate struct {
	OrgID                uuid.UUID
	StripeCustomerID     *string
	StripeSubscriptionID *string
	BillingStatus        billingdomain.BillingStatus
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetTenantBilling(ctx context.Context, orgID uuid.UUID) (billingdomain.TenantBilling, bool, error) {
	var row models.TenantSettings
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return billingdomain.TenantBilling{}, false, nil
		}
		return billingdomain.TenantBilling{}, false, err
	}
	out, err := toTenantBillingDomain(row)
	if err != nil {
		return billingdomain.TenantBilling{}, false, err
	}
	return out, true, nil
}

func (r *Repository) UpdateStripeFields(ctx context.Context, in stripeFieldsUpdate) error {
	assign := map[string]any{
		"billing_status": in.BillingStatus,
	}
	if in.StripeCustomerID != nil {
		assign["stripe_customer_id"] = *in.StripeCustomerID
	}
	if in.StripeSubscriptionID != nil {
		assign["stripe_subscription_id"] = *in.StripeSubscriptionID
	}
	tx := r.db.WithContext(ctx).Model(&models.TenantSettings{}).Where("org_id = ?", in.OrgID).Updates(assign)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return types.NewHTTPError(404, types.ErrCodeNotFound, "tenant settings not found")
	}
	return nil
}

func (r *Repository) ClearSubscription(ctx context.Context, orgID uuid.UUID, status billingdomain.BillingStatus) error {
	tx := r.db.WithContext(ctx).Model(&models.TenantSettings{}).Where("org_id = ?", orgID).Updates(map[string]any{
		"stripe_subscription_id": nil,
		"billing_status":         status,
	})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return types.NewHTTPError(404, types.ErrCodeNotFound, "tenant settings not found")
	}
	return nil
}

func (r *Repository) UpdateBillingStatusByOrgID(ctx context.Context, orgID uuid.UUID, status billingdomain.BillingStatus) error {
	tx := r.db.WithContext(ctx).Model(&models.TenantSettings{}).Where("org_id = ?", orgID).Update("billing_status", status)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return types.NewHTTPError(404, types.ErrCodeNotFound, "tenant settings not found")
	}
	return nil
}

func (r *Repository) FindOrgIDBySubscriptionID(ctx context.Context, subscriptionID string) (uuid.UUID, bool, error) {
	var row struct {
		OrgID uuid.UUID `gorm:"column:org_id"`
	}
	err := r.db.WithContext(ctx).
		Table("tenant_settings").
		Select("org_id").
		Where("stripe_subscription_id = ?", subscriptionID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}
	return row.OrgID, true, nil
}

func (r *Repository) FindOrgIDByCustomerID(ctx context.Context, customerID string) (uuid.UUID, bool, error) {
	var row struct {
		OrgID uuid.UUID `gorm:"column:org_id"`
	}
	err := r.db.WithContext(ctx).
		Table("tenant_settings").
		Select("org_id").
		Where("stripe_customer_id = ?", customerID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}
	return row.OrgID, true, nil
}

func (r *Repository) GetOrgName(ctx context.Context, orgID uuid.UUID) (string, error) {
	var row models.Org
	if err := r.db.WithContext(ctx).Where("id = ?", orgID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", types.NewHTTPError(404, types.ErrCodeNotFound, "org not found")
		}
		return "", err
	}
	return row.Name, nil
}

func (r *Repository) GetUserEmailByExternalID(ctx context.Context, externalID string) (string, bool, error) {
	var row struct {
		Email string `gorm:"column:email"`
	}
	err := r.db.WithContext(ctx).
		Table("users").
		Select("email").
		Where("external_id = ?", externalID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return row.Email, true, nil
}

func (r *Repository) GetUsageSummary(ctx context.Context, orgID uuid.UUID, period time.Time) (billingdomain.UsageSummary, error) {
	type row struct {
		Counter string `gorm:"column:counter"`
		Value   int64  `gorm:"column:value"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("org_usage_counters").
		Select("counter, value").
		Where("org_id = ? AND period = ?", orgID, period.Format("2006-01-02")).
		Find(&rows).Error; err != nil {
		return billingdomain.UsageSummary{}, err
	}

	out := billingdomain.UsageSummary{
		Period: period.Format("2006-01"),
		Counters: billingdomain.UsageCounters{
			APICalls:        0,
			EventsIngested:  0,
			IncidentsOpened: 0,
			ActionsExecuted: 0,
		},
	}
	for _, item := range rows {
		switch item.Counter {
		case usagemetering.CounterAPICalls:
			out.Counters.APICalls = item.Value
		case usagemetering.CounterEventsIngested:
			out.Counters.EventsIngested = item.Value
		case usagemetering.CounterIncidentsOpened:
			out.Counters.IncidentsOpened = item.Value
		case usagemetering.CounterActionsExecuted:
			out.Counters.ActionsExecuted = item.Value
		}
	}
	return out, nil
}

func toTenantBillingDomain(in models.TenantSettings) (billingdomain.TenantBilling, error) {
	limits, err := decodeHardLimits(in.HardLimits)
	if err != nil {
		return billingdomain.TenantBilling{}, err
	}
	return billingdomain.TenantBilling{
		OrgID:                in.OrgID,
		PlanCode:             billingdomain.PlanCode(in.PlanCode),
		HardLimits:           limits,
		BillingStatus:        billingdomain.BillingStatus(in.BillingStatus),
		StripeCustomerID:     in.StripeCustomerID,
		StripeSubscriptionID: in.StripeSubscriptionID,
		UpdatedAt:            in.UpdatedAt,
		CreatedAt:            in.CreatedAt,
	}, nil
}

func decodeHardLimits(raw []byte) (billingdomain.HardLimits, error) {
	if len(raw) == 0 {
		return billingdomain.HardLimits{}, nil
	}
	var out billingdomain.HardLimits
	if err := json.Unmarshal(raw, &out); err != nil {
		return billingdomain.HardLimits{}, err
	}
	return out, nil
}
