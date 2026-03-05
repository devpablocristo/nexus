package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"

	"nexus-saas/cmd/config"
	"nexus-saas/internal/admin"
	admindomain "nexus-saas/internal/admin/usecases/domain"
	billingdomain "nexus-saas/internal/billing/usecases/domain"
	"nexus/pkg/types"
)

type TenantSettingsPort interface {
	UpsertTenantSettings(ctx context.Context, s admindomain.TenantSettings) (admindomain.TenantSettings, error)
}

type Usecases struct {
	repo            *Repository
	tenantSettings  TenantSettingsPort
	stripe          StripeClientPort
	stripeEnabled   bool
	webhookSecret   string
	priceStarter    string
	priceGrowth     string
	priceEnterprise string
	towerBaseURL    string
}

func NewUsecases(cfg config.ServiceConfig, repo *Repository, tenantSettings TenantSettingsPort, stripeClient *StripeClient) *Usecases {
	return &Usecases{
		repo:            repo,
		tenantSettings:  tenantSettings,
		stripe:          stripeClient,
		stripeEnabled:   strings.TrimSpace(cfg.StripeSecretKey) != "",
		webhookSecret:   strings.TrimSpace(cfg.StripeWebhookSecret),
		priceStarter:    strings.TrimSpace(cfg.StripePriceStarter),
		priceGrowth:     strings.TrimSpace(cfg.StripePriceGrowth),
		priceEnterprise: strings.TrimSpace(cfg.StripePriceEnterprise),
		towerBaseURL:    sanitizeBaseURL(cfg.TowerBaseURL),
	}
}

func (u *Usecases) Enabled() bool {
	return u.stripeEnabled
}

func (u *Usecases) WebhookSecret() string {
	return u.webhookSecret
}

func (u *Usecases) GetBillingStatus(ctx context.Context, orgID uuid.UUID) (billingdomain.BillingStatusView, error) {
	if err := u.requireStripeEnabled(); err != nil {
		return billingdomain.BillingStatusView{}, err
	}
	settings, err := u.ensureTenantSettings(ctx, orgID)
	if err != nil {
		return billingdomain.BillingStatusView{}, err
	}
	usage, err := u.GetUsageSummary(ctx, orgID)
	if err != nil {
		return billingdomain.BillingStatusView{}, err
	}
	var currentPeriodEnd *time.Time
	if settings.StripeSubscriptionID != nil && *settings.StripeSubscriptionID != "" {
		sub, err := u.stripe.GetSubscription(*settings.StripeSubscriptionID)
		if err == nil && sub != nil && sub.CurrentPeriodEnd > 0 {
			ts := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
			currentPeriodEnd = &ts
		}
	}
	return billingdomain.BillingStatusView{
		PlanCode:         settings.PlanCode,
		BillingStatus:    settings.BillingStatus,
		CurrentPeriodEnd: currentPeriodEnd,
		HardLimits:       settings.HardLimits,
		Usage:            usage,
	}, nil
}

func (u *Usecases) CreateCheckoutSession(ctx context.Context, orgID uuid.UUID, planCode string, successURL, cancelURL string, actor *string) (string, error) {
	if err := u.requireStripeEnabled(); err != nil {
		return "", err
	}
	plan := normalizePlanCode(planCode)
	if plan == "" {
		return "", types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "plan_code must be starter|growth|enterprise")
	}
	priceID := u.priceIDByPlan(plan)
	if priceID == "" {
		return "", types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeInternal, "stripe price not configured for plan")
	}
	successURL = strings.TrimSpace(successURL)
	cancelURL = strings.TrimSpace(cancelURL)
	if successURL == "" {
		successURL = u.towerBaseURL + "/billing/success?plan=" + string(plan) + "&session_id={CHECKOUT_SESSION_ID}"
	}
	if cancelURL == "" {
		cancelURL = u.towerBaseURL + "/billing?canceled=1"
	}
	if _, err := url.ParseRequestURI(successURL); err != nil {
		return "", types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid success_url")
	}
	if _, err := url.ParseRequestURI(cancelURL); err != nil {
		return "", types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid cancel_url")
	}

	settings, err := u.ensureTenantSettings(ctx, orgID)
	if err != nil {
		return "", err
	}
	customerID, err := u.ensureStripeCustomer(ctx, settings, actor)
	if err != nil {
		return "", err
	}

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Customer:   stripe.String(customerID),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"org_id":    orgID.String(),
			"plan_code": string(plan),
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"org_id":    orgID.String(),
				"plan_code": string(plan),
			},
		},
	}
	session, err := u.stripe.CreateCheckoutSession(params)
	if err != nil {
		return "", err
	}
	if session == nil || strings.TrimSpace(session.URL) == "" {
		return "", types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeInternal, "stripe checkout session missing url")
	}
	return session.URL, nil
}

func (u *Usecases) CreatePortalSession(ctx context.Context, orgID uuid.UUID, returnURL string, actor *string) (string, error) {
	if err := u.requireStripeEnabled(); err != nil {
		return "", err
	}
	returnURL = strings.TrimSpace(returnURL)
	if returnURL == "" {
		returnURL = u.towerBaseURL + "/billing"
	}
	if _, err := url.ParseRequestURI(returnURL); err != nil {
		return "", types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid return_url")
	}

	settings, err := u.ensureTenantSettings(ctx, orgID)
	if err != nil {
		return "", err
	}
	customerID, err := u.ensureStripeCustomer(ctx, settings, actor)
	if err != nil {
		return "", err
	}
	session, err := u.stripe.CreatePortalSession(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", err
	}
	if session == nil || strings.TrimSpace(session.URL) == "" {
		return "", types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeInternal, "stripe portal session missing url")
	}
	return session.URL, nil
}

func (u *Usecases) GetUsageSummary(ctx context.Context, orgID uuid.UUID) (billingdomain.UsageSummary, error) {
	if err := u.requireStripeEnabled(); err != nil {
		return billingdomain.UsageSummary{}, err
	}
	return u.repo.GetUsageSummary(ctx, orgID, billingPeriodUTC(time.Now().UTC()))
}

func (u *Usecases) HandleWebhookEvent(ctx context.Context, event stripe.Event) error {
	if !u.Enabled() {
		return types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeInternal, "stripe billing is not configured")
	}
	if event.Data == nil {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "stripe webhook event missing data")
	}
	switch event.Type {
	case "checkout.session.completed":
		return u.handleCheckoutCompleted(ctx, event.Data.Raw)
	case "customer.subscription.updated":
		return u.handleSubscriptionUpdated(ctx, event.Data.Raw)
	case "customer.subscription.deleted":
		return u.handleSubscriptionDeleted(ctx, event.Data.Raw)
	case "invoice.payment_succeeded":
		return u.handleInvoicePayment(ctx, event.Data.Raw, billingdomain.BillingActive)
	case "invoice.payment_failed":
		return u.handleInvoicePayment(ctx, event.Data.Raw, billingdomain.BillingPastDue)
	default:
		return nil
	}
}

func (u *Usecases) handleCheckoutCompleted(ctx context.Context, raw json.RawMessage) error {
	var payload struct {
		Customer     string            `json:"customer"`
		Subscription string            `json:"subscription"`
		Metadata     map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	orgID, err := uuid.Parse(strings.TrimSpace(payload.Metadata["org_id"]))
	if err != nil {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "checkout metadata missing org_id")
	}
	plan := normalizePlanCode(payload.Metadata["plan_code"])
	if plan == "" {
		plan = billingdomain.PlanStarter
	}
	return u.applySubscriptionState(ctx, orgID, plan, billingdomain.BillingActive, payload.Customer, payload.Subscription)
}

func (u *Usecases) handleSubscriptionUpdated(ctx context.Context, raw json.RawMessage) error {
	var payload struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
		Status   string `json:"status"`
		Items    struct {
			Data []struct {
				Price struct {
					ID string `json:"id"`
				} `json:"price"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	orgID, ok, err := u.repo.FindOrgIDBySubscriptionID(ctx, payload.ID)
	if err != nil {
		return err
	}
	if !ok {
		orgID, ok, err = u.repo.FindOrgIDByCustomerID(ctx, payload.Customer)
		if err != nil {
			return err
		}
	}
	if !ok {
		return nil
	}
	plan := billingdomain.PlanStarter
	if len(payload.Items.Data) > 0 {
		plan = u.planByPriceID(payload.Items.Data[0].Price.ID)
	}
	status := billingStatusFromStripe(payload.Status)
	return u.applySubscriptionState(ctx, orgID, plan, status, payload.Customer, payload.ID)
}

func (u *Usecases) handleSubscriptionDeleted(ctx context.Context, raw json.RawMessage) error {
	var payload struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	orgID, ok, err := u.repo.FindOrgIDBySubscriptionID(ctx, payload.ID)
	if err != nil {
		return err
	}
	if !ok {
		orgID, ok, err = u.repo.FindOrgIDByCustomerID(ctx, payload.Customer)
		if err != nil {
			return err
		}
	}
	if !ok {
		return nil
	}
	if err := u.applyPlanSettings(ctx, orgID, billingdomain.PlanStarter); err != nil {
		return err
	}
	return u.repo.ClearSubscription(ctx, orgID, billingdomain.BillingCanceled)
}

func (u *Usecases) handleInvoicePayment(ctx context.Context, raw json.RawMessage, status billingdomain.BillingStatus) error {
	var payload struct {
		Subscription string `json:"subscription"`
		Customer     string `json:"customer"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	orgID, ok, err := u.repo.FindOrgIDBySubscriptionID(ctx, payload.Subscription)
	if err != nil {
		return err
	}
	if !ok {
		orgID, ok, err = u.repo.FindOrgIDByCustomerID(ctx, payload.Customer)
		if err != nil {
			return err
		}
	}
	if !ok {
		return nil
	}
	return u.repo.UpdateBillingStatusByOrgID(ctx, orgID, status)
}

func (u *Usecases) applySubscriptionState(
	ctx context.Context,
	orgID uuid.UUID,
	plan billingdomain.PlanCode,
	status billingdomain.BillingStatus,
	customerID, subscriptionID string,
) error {
	settings, err := u.ensureTenantSettings(ctx, orgID)
	if err != nil {
		return err
	}
	effectiveCustomer := pick(customerID, valueOrEmpty(settings.StripeCustomerID))
	effectiveSubscription := pick(subscriptionID, valueOrEmpty(settings.StripeSubscriptionID))

	if settings.PlanCode != plan {
		if err := u.applyPlanSettings(ctx, orgID, plan); err != nil {
			return err
		}
	}
	if settings.BillingStatus == status &&
		valueOrEmpty(settings.StripeCustomerID) == effectiveCustomer &&
		valueOrEmpty(settings.StripeSubscriptionID) == effectiveSubscription {
		return nil
	}

	update := stripeFieldsUpdate{
		OrgID:         orgID,
		BillingStatus: status,
	}
	if effectiveCustomer != "" {
		update.StripeCustomerID = &effectiveCustomer
	}
	if effectiveSubscription != "" {
		update.StripeSubscriptionID = &effectiveSubscription
	}
	return u.repo.UpdateStripeFields(ctx, update)
}

func (u *Usecases) ensureTenantSettings(ctx context.Context, orgID uuid.UUID) (billingdomain.TenantBilling, error) {
	settings, ok, err := u.repo.GetTenantBilling(ctx, orgID)
	if err != nil {
		return billingdomain.TenantBilling{}, err
	}
	if ok {
		return settings, nil
	}
	_, err = u.tenantSettings.UpsertTenantSettings(ctx, admindomain.TenantSettings{
		OrgID:      orgID,
		PlanCode:   string(billingdomain.PlanStarter),
		HardLimits: admin.DefaultHardLimits(string(billingdomain.PlanStarter)),
		UpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		return billingdomain.TenantBilling{}, err
	}
	settings, ok, err = u.repo.GetTenantBilling(ctx, orgID)
	if err != nil {
		return billingdomain.TenantBilling{}, err
	}
	if !ok {
		return billingdomain.TenantBilling{}, types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeInternal, "failed to initialize tenant billing settings")
	}
	return settings, nil
}

func (u *Usecases) ensureStripeCustomer(ctx context.Context, settings billingdomain.TenantBilling, actor *string) (string, error) {
	if settings.StripeCustomerID != nil && strings.TrimSpace(*settings.StripeCustomerID) != "" {
		return strings.TrimSpace(*settings.StripeCustomerID), nil
	}
	orgName, err := u.repo.GetOrgName(ctx, settings.OrgID)
	if err != nil {
		return "", err
	}
	var email *string
	if actor != nil && strings.TrimSpace(*actor) != "" {
		if resolved, ok, err := u.repo.GetUserEmailByExternalID(ctx, strings.TrimSpace(*actor)); err != nil {
			return "", err
		} else if ok {
			email = &resolved
		}
	}
	params := &stripe.CustomerParams{
		Name: stripe.String(orgName),
		Metadata: map[string]string{
			"org_id": settings.OrgID.String(),
		},
	}
	if email != nil {
		params.Email = stripe.String(*email)
	}
	customer, err := u.stripe.CreateCustomer(params)
	if err != nil {
		return "", err
	}
	if customer == nil || strings.TrimSpace(customer.ID) == "" {
		return "", types.NewHTTPError(http.StatusInternalServerError, types.ErrCodeInternal, "stripe customer missing id")
	}
	customerID := strings.TrimSpace(customer.ID)
	if err := u.repo.UpdateStripeFields(ctx, stripeFieldsUpdate{
		OrgID:            settings.OrgID,
		StripeCustomerID: &customerID,
		BillingStatus:    settings.BillingStatus,
	}); err != nil {
		return "", err
	}
	return customerID, nil
}

func (u *Usecases) applyPlanSettings(ctx context.Context, orgID uuid.UUID, plan billingdomain.PlanCode) error {
	_, err := u.tenantSettings.UpsertTenantSettings(ctx, admindomain.TenantSettings{
		OrgID:      orgID,
		PlanCode:   string(plan),
		HardLimits: admin.DefaultHardLimits(string(plan)),
		UpdatedAt:  time.Now().UTC(),
	})
	return err
}

func (u *Usecases) priceIDByPlan(plan billingdomain.PlanCode) string {
	switch plan {
	case billingdomain.PlanStarter:
		return u.priceStarter
	case billingdomain.PlanGrowth:
		return u.priceGrowth
	case billingdomain.PlanEnterprise:
		return u.priceEnterprise
	default:
		return ""
	}
}

func (u *Usecases) planByPriceID(priceID string) billingdomain.PlanCode {
	priceID = strings.TrimSpace(priceID)
	switch priceID {
	case u.priceStarter:
		return billingdomain.PlanStarter
	case u.priceGrowth:
		return billingdomain.PlanGrowth
	case u.priceEnterprise:
		return billingdomain.PlanEnterprise
	default:
		return billingdomain.PlanStarter
	}
}

func (u *Usecases) requireStripeEnabled() error {
	if !u.Enabled() {
		return types.NewHTTPError(http.StatusServiceUnavailable, types.ErrCodeInternal, "stripe billing is not configured")
	}
	return nil
}

func normalizePlanCode(raw string) billingdomain.PlanCode {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(billingdomain.PlanStarter):
		return billingdomain.PlanStarter
	case string(billingdomain.PlanGrowth):
		return billingdomain.PlanGrowth
	case string(billingdomain.PlanEnterprise):
		return billingdomain.PlanEnterprise
	default:
		return ""
	}
}

func billingStatusFromStripe(raw string) billingdomain.BillingStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "active":
		return billingdomain.BillingActive
	case "past_due":
		return billingdomain.BillingPastDue
	case "canceled":
		return billingdomain.BillingCanceled
	case "unpaid":
		return billingdomain.BillingUnpaid
	default:
		return billingdomain.BillingTrialing
	}
}

func billingPeriodUTC(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func sanitizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "http://localhost:5173"
	}
	return strings.TrimRight(raw, "/")
}

func pick(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return strings.TrimSpace(fallback)
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func (u *Usecases) String() string {
	return fmt.Sprintf("billing(enabled=%t)", u.Enabled())
}
