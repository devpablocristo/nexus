package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	admindomain "control-plane/internal/admin/usecases/domain"
	billingdomain "control-plane/internal/billing/usecases/domain"
	"control-plane/internal/usagemetering"
	"nexus/pkg/types"
)

func TestUsecases_CreateCheckoutSessionCreatesCustomerAndReturnsURL(t *testing.T) {
	db := newBillingTestDB(t)
	repo := NewRepository(db)
	port := &fakeTenantSettingsPort{db: db}

	orgID := uuid.New()
	seedOrg(t, db, orgID, "Acme")
	seedUser(t, db, "user_ext_1", "owner@acme.test")

	var capturedCustomerEmail string
	var capturedCustomerName string
	var capturedPlan string
	var capturedOrgMetadata string
	stripeFake := &fakeStripeClient{
		createCustomerFn: func(params *stripe.CustomerParams) (*stripe.Customer, error) {
			if params.Email != nil {
				capturedCustomerEmail = *params.Email
			}
			if params.Name != nil {
				capturedCustomerName = *params.Name
			}
			capturedOrgMetadata = params.Metadata["org_id"]
			return &stripe.Customer{ID: "cus_test_001"}, nil
		},
		createCheckoutSessionFn: func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
			capturedPlan = params.Metadata["plan_code"]
			return &stripe.CheckoutSession{URL: "https://stripe.test/checkout/abc"}, nil
		},
	}

	uc := &Usecases{
		repo:            repo,
		tenantSettings:  port,
		stripe:          stripeFake,
		stripeEnabled:   true,
		priceStarter:    "price_starter",
		priceGrowth:     "price_growth",
		priceEnterprise: "price_enterprise",
		towerBaseURL:    "http://localhost:5173",
	}

	gotURL, err := uc.CreateCheckoutSession(context.Background(), orgID, "growth", "", "", strPtr("user_ext_1"))
	if err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if gotURL != "https://stripe.test/checkout/abc" {
		t.Fatalf("unexpected checkout url: %s", gotURL)
	}
	if capturedCustomerEmail != "owner@acme.test" {
		t.Fatalf("unexpected customer email: %s", capturedCustomerEmail)
	}
	if capturedCustomerName != "Acme" {
		t.Fatalf("unexpected customer name: %s", capturedCustomerName)
	}
	if capturedOrgMetadata != orgID.String() {
		t.Fatalf("unexpected org metadata: %s", capturedOrgMetadata)
	}
	if capturedPlan != "growth" {
		t.Fatalf("unexpected plan metadata: %s", capturedPlan)
	}
	stored := mustGetTenantBilling(t, repo, orgID)
	if valueOrEmpty(stored.StripeCustomerID) != "cus_test_001" {
		t.Fatalf("customer id not persisted")
	}
	if port.upsertCalls == 0 {
		t.Fatalf("expected tenant settings bootstrap")
	}
}

func TestUsecases_GetBillingStatusIncludesUsageAndPeriodEnd(t *testing.T) {
	db := newBillingTestDB(t)
	repo := NewRepository(db)
	port := &fakeTenantSettingsPort{db: db}

	orgID := uuid.New()
	seedOrg(t, db, orgID, "Umbrella")
	seedTenantSettings(t, db, orgID, tenantSeed{
		PlanCode:       "growth",
		BillingStatus:  "active",
		CustomerID:     strPtr("cus_123"),
		SubscriptionID: strPtr("sub_123"),
		HardLimits: map[string]any{
			"tools_max": 75, "run_rpm": 1200, "audit_retention_days": 90,
		},
	})
	seedUsageCounter(t, db, orgID, usagemetering.CounterAPICalls, 12450)
	seedUsageCounter(t, db, orgID, usagemetering.CounterEventsIngested, 3200)

	expectedPeriodEnd := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	stripeFake := &fakeStripeClient{
		getSubscriptionFn: func(subscriptionID string) (*stripe.Subscription, error) {
			if subscriptionID != "sub_123" {
				t.Fatalf("unexpected subscription id: %s", subscriptionID)
			}
			return &stripe.Subscription{CurrentPeriodEnd: expectedPeriodEnd.Unix()}, nil
		},
	}

	uc := &Usecases{
		repo:           repo,
		tenantSettings: port,
		stripe:         stripeFake,
		stripeEnabled:  true,
	}

	view, err := uc.GetBillingStatus(context.Background(), orgID)
	if err != nil {
		t.Fatalf("GetBillingStatus: %v", err)
	}
	if view.PlanCode != "growth" {
		t.Fatalf("unexpected plan: %s", view.PlanCode)
	}
	if view.BillingStatus != "active" {
		t.Fatalf("unexpected billing status: %s", view.BillingStatus)
	}
	if view.CurrentPeriodEnd == nil {
		t.Fatalf("expected current_period_end")
	}
	if view.CurrentPeriodEnd.Unix() != expectedPeriodEnd.Unix() {
		t.Fatalf("unexpected current_period_end: %s", view.CurrentPeriodEnd)
	}
	if view.Usage.Counters.APICalls != 12450 {
		t.Fatalf("unexpected api_calls: %d", view.Usage.Counters.APICalls)
	}
	if view.Usage.Counters.EventsIngested != 3200 {
		t.Fatalf("unexpected events_ingested: %d", view.Usage.Counters.EventsIngested)
	}
}

func TestUsecases_HandleWebhookEventsLifecycle(t *testing.T) {
	db := newBillingTestDB(t)
	repo := NewRepository(db)
	port := &fakeTenantSettingsPort{db: db}

	orgID := uuid.New()
	seedOrg(t, db, orgID, "Nexus")
	seedTenantSettings(t, db, orgID, tenantSeed{
		PlanCode:       "starter",
		BillingStatus:  "trialing",
		CustomerID:     strPtr("cus_seed"),
		SubscriptionID: strPtr("sub_seed"),
		HardLimits: map[string]any{
			"tools_max": 20, "run_rpm": 300, "audit_retention_days": 30,
		},
	})

	uc := &Usecases{
		repo:            repo,
		tenantSettings:  port,
		stripe:          &fakeStripeClient{},
		stripeEnabled:   true,
		priceStarter:    "price_starter",
		priceGrowth:     "price_growth",
		priceEnterprise: "price_enterprise",
	}

	// checkout.session.completed -> growth + active
	eventCheckout := stripe.Event{
		Type: stripe.EventType("checkout.session.completed"),
		Data: &stripe.EventData{Raw: json.RawMessage(fmt.Sprintf(`{
			"customer":"cus_live",
			"subscription":"sub_live",
			"metadata":{"org_id":"%s","plan_code":"growth"}
		}`, orgID.String()))},
	}
	if err := uc.HandleWebhookEvent(context.Background(), eventCheckout); err != nil {
		t.Fatalf("HandleWebhookEvent(checkout): %v", err)
	}
	stored := mustGetTenantBilling(t, repo, orgID)
	if stored.PlanCode != "growth" || stored.BillingStatus != "active" {
		t.Fatalf("unexpected state after checkout: plan=%s status=%s", stored.PlanCode, stored.BillingStatus)
	}
	if valueOrEmpty(stored.StripeCustomerID) != "cus_live" || valueOrEmpty(stored.StripeSubscriptionID) != "sub_live" {
		t.Fatalf("unexpected stripe ids after checkout")
	}

	// customer.subscription.updated -> enterprise + unpaid
	eventSubUpdated := stripe.Event{
		Type: stripe.EventType("customer.subscription.updated"),
		Data: &stripe.EventData{Raw: json.RawMessage(`{
			"id":"sub_live",
			"customer":"cus_live",
			"status":"unpaid",
			"items":{"data":[{"price":{"id":"price_enterprise"}}]}
		}`)},
	}
	if err := uc.HandleWebhookEvent(context.Background(), eventSubUpdated); err != nil {
		t.Fatalf("HandleWebhookEvent(subscription.updated): %v", err)
	}
	stored = mustGetTenantBilling(t, repo, orgID)
	if stored.PlanCode != "enterprise" || stored.BillingStatus != "unpaid" {
		t.Fatalf("unexpected state after subscription.updated: plan=%s status=%s", stored.PlanCode, stored.BillingStatus)
	}

	// invoice.payment_succeeded -> active
	eventInvoiceOK := stripe.Event{
		Type: stripe.EventType("invoice.payment_succeeded"),
		Data: &stripe.EventData{Raw: json.RawMessage(`{"subscription":"sub_live","customer":"cus_live"}`)},
	}
	if err := uc.HandleWebhookEvent(context.Background(), eventInvoiceOK); err != nil {
		t.Fatalf("HandleWebhookEvent(invoice.payment_succeeded): %v", err)
	}
	stored = mustGetTenantBilling(t, repo, orgID)
	if stored.BillingStatus != "active" {
		t.Fatalf("expected active after invoice.payment_succeeded, got %s", stored.BillingStatus)
	}

	// invoice.payment_failed -> past_due
	eventInvoiceFailed := stripe.Event{
		Type: stripe.EventType("invoice.payment_failed"),
		Data: &stripe.EventData{Raw: json.RawMessage(`{"subscription":"sub_live","customer":"cus_live"}`)},
	}
	if err := uc.HandleWebhookEvent(context.Background(), eventInvoiceFailed); err != nil {
		t.Fatalf("HandleWebhookEvent(invoice.payment_failed): %v", err)
	}
	stored = mustGetTenantBilling(t, repo, orgID)
	if stored.BillingStatus != "past_due" {
		t.Fatalf("expected past_due after invoice.payment_failed, got %s", stored.BillingStatus)
	}
	if stored.PastDueSince == nil {
		t.Fatalf("expected past_due_since to be set after invoice.payment_failed")
	}

	// invoice.payment_succeeded -> active + clear past_due_since
	if err := uc.HandleWebhookEvent(context.Background(), eventInvoiceOK); err != nil {
		t.Fatalf("HandleWebhookEvent(invoice.payment_succeeded second): %v", err)
	}
	stored = mustGetTenantBilling(t, repo, orgID)
	if stored.BillingStatus != "active" {
		t.Fatalf("expected active after second invoice.payment_succeeded, got %s", stored.BillingStatus)
	}
	if stored.PastDueSince != nil {
		t.Fatalf("expected past_due_since to be cleared after payment recovery")
	}

	// customer.subscription.deleted -> starter + canceled + clear subscription
	eventSubDeleted := stripe.Event{
		Type: stripe.EventType("customer.subscription.deleted"),
		Data: &stripe.EventData{Raw: json.RawMessage(`{"id":"sub_live","customer":"cus_live"}`)},
	}
	if err := uc.HandleWebhookEvent(context.Background(), eventSubDeleted); err != nil {
		t.Fatalf("HandleWebhookEvent(subscription.deleted): %v", err)
	}
	stored = mustGetTenantBilling(t, repo, orgID)
	if stored.PlanCode != "starter" || stored.BillingStatus != "canceled" {
		t.Fatalf("unexpected state after subscription.deleted: plan=%s status=%s", stored.PlanCode, stored.BillingStatus)
	}
	if valueOrEmpty(stored.StripeSubscriptionID) != "" {
		t.Fatalf("expected cleared stripe subscription id")
	}
	if stored.HardLimits.ToolsMax != 20 || stored.HardLimits.RunRPM != 300 || stored.HardLimits.AuditRetentionDays != 30 {
		t.Fatalf("starter hard limits were not restored: %+v", stored.HardLimits)
	}
}

func TestUsecases_HandleWebhookCheckoutIsIdempotent(t *testing.T) {
	db := newBillingTestDB(t)
	repo := NewRepository(db)
	port := &fakeTenantSettingsPort{db: db}

	orgID := uuid.New()
	seedOrg(t, db, orgID, "Wayne")
	seedTenantSettings(t, db, orgID, tenantSeed{
		PlanCode:       "growth",
		BillingStatus:  "active",
		CustomerID:     strPtr("cus_idem"),
		SubscriptionID: strPtr("sub_idem"),
		HardLimits: map[string]any{
			"tools_max": 75, "run_rpm": 1200, "audit_retention_days": 90,
		},
	})
	port.upsertCalls = 0

	uc := &Usecases{
		repo:           repo,
		tenantSettings: port,
		stripe:         &fakeStripeClient{},
		stripeEnabled:  true,
	}

	event := stripe.Event{
		Type: stripe.EventType("checkout.session.completed"),
		Data: &stripe.EventData{Raw: json.RawMessage(fmt.Sprintf(`{
			"customer":"cus_idem",
			"subscription":"sub_idem",
			"metadata":{"org_id":"%s","plan_code":"growth"}
		}`, orgID.String()))},
	}
	if err := uc.HandleWebhookEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleWebhookEvent: %v", err)
	}
	if port.upsertCalls != 0 {
		t.Fatalf("expected no plan upsert for idempotent webhook, got %d", port.upsertCalls)
	}
}

func TestUsecases_RequireStripeEnabled(t *testing.T) {
	db := newBillingTestDB(t)
	repo := NewRepository(db)
	uc := &Usecases{
		repo:           repo,
		tenantSettings: &fakeTenantSettingsPort{db: db},
		stripe:         &fakeStripeClient{},
		stripeEnabled:  false,
	}
	_, err := uc.GetUsageSummary(context.Background(), uuid.New())
	if err == nil {
		t.Fatalf("expected error when stripe is disabled")
	}
	httpErr, ok := err.(types.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", httpErr.Status)
	}
}

type fakeStripeClient struct {
	createCustomerFn        func(params *stripe.CustomerParams) (*stripe.Customer, error)
	createCheckoutSessionFn func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	createPortalSessionFn   func(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
	getSubscriptionFn       func(subscriptionID string) (*stripe.Subscription, error)
	constructWebhookEventFn func(payload []byte, sigHeader, secret string) (stripe.Event, error)
}

func (f *fakeStripeClient) CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error) {
	if f.createCustomerFn != nil {
		return f.createCustomerFn(params)
	}
	return &stripe.Customer{ID: "cus_default"}, nil
}

func (f *fakeStripeClient) CreateCheckoutSession(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	if f.createCheckoutSessionFn != nil {
		return f.createCheckoutSessionFn(params)
	}
	return &stripe.CheckoutSession{URL: "https://stripe.test/default-checkout"}, nil
}

func (f *fakeStripeClient) CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	if f.createPortalSessionFn != nil {
		return f.createPortalSessionFn(params)
	}
	return &stripe.BillingPortalSession{URL: "https://stripe.test/default-portal"}, nil
}

func (f *fakeStripeClient) GetSubscription(subscriptionID string) (*stripe.Subscription, error) {
	if f.getSubscriptionFn != nil {
		return f.getSubscriptionFn(subscriptionID)
	}
	return &stripe.Subscription{CurrentPeriodEnd: time.Now().UTC().Add(24 * time.Hour).Unix()}, nil
}

func (f *fakeStripeClient) ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error) {
	if f.constructWebhookEventFn != nil {
		return f.constructWebhookEventFn(payload, sigHeader, secret)
	}
	return stripe.Event{Type: stripe.EventType("noop"), Data: &stripe.EventData{Raw: payload}}, nil
}

type fakeTenantSettingsPort struct {
	db          *gorm.DB
	upsertCalls int
}

func (f *fakeTenantSettingsPort) UpsertTenantSettings(ctx context.Context, s admindomain.TenantSettings) (admindomain.TenantSettings, error) {
	f.upsertCalls++
	raw, err := json.Marshal(s.HardLimits)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	if err := f.db.WithContext(ctx).Exec(`
		INSERT INTO tenant_settings(org_id, plan_code, hard_limits_json, updated_by, updated_at, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(org_id) DO UPDATE SET
			plan_code=excluded.plan_code,
			hard_limits_json=excluded.hard_limits_json,
			updated_by=excluded.updated_by,
			updated_at=CURRENT_TIMESTAMP
	`, s.OrgID.String(), s.PlanCode, string(raw), nullableString(s.UpdatedBy)).Error; err != nil {
		return admindomain.TenantSettings{}, err
	}
	return s, nil
}

type tenantSeed struct {
	PlanCode       string
	BillingStatus  string
	CustomerID     *string
	SubscriptionID *string
	HardLimits     map[string]any
}

func newBillingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	stmts := []string{
		`CREATE TABLE orgs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			external_id TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			avatar_url TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE tenant_settings (
				org_id TEXT PRIMARY KEY,
				plan_code TEXT NOT NULL,
				hard_limits_json TEXT NOT NULL DEFAULT '{}',
				stripe_customer_id TEXT UNIQUE,
				stripe_subscription_id TEXT UNIQUE,
				billing_status TEXT NOT NULL DEFAULT 'trialing',
				past_due_since DATETIME NULL,
				updated_by TEXT NULL,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
		`CREATE TABLE org_usage_counters (
			org_id TEXT NOT NULL,
			period DATE NOT NULL,
			counter TEXT NOT NULL,
			value INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(org_id, period, counter)
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func seedOrg(t *testing.T, db *gorm.DB, orgID uuid.UUID, name string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO orgs(id, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, orgID.String(), name).Error; err != nil {
		t.Fatalf("seed org: %v", err)
	}
}

func seedUser(t *testing.T, db *gorm.DB, externalID, email string) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO users(id, external_id, email, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, uuid.NewString(), externalID, email, "Owner").Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedTenantSettings(t *testing.T, db *gorm.DB, orgID uuid.UUID, in tenantSeed) {
	t.Helper()
	raw, err := json.Marshal(in.HardLimits)
	if err != nil {
		t.Fatalf("marshal hard limits: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO tenant_settings(
			org_id, plan_code, hard_limits_json, stripe_customer_id, stripe_subscription_id, billing_status, updated_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, orgID.String(), in.PlanCode, string(raw), nullableString(in.CustomerID), nullableString(in.SubscriptionID), in.BillingStatus).Error; err != nil {
		t.Fatalf("seed tenant settings: %v", err)
	}
}

func seedUsageCounter(t *testing.T, db *gorm.DB, orgID uuid.UUID, counter string, value int64) {
	t.Helper()
	period := billingPeriodUTC(time.Now().UTC()).Format("2006-01-02")
	if err := db.Exec(`
		INSERT INTO org_usage_counters(org_id, period, counter, value, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, orgID.String(), period, counter, value).Error; err != nil {
		t.Fatalf("seed usage counter: %v", err)
	}
}

func mustGetTenantBilling(t *testing.T, repo *Repository, orgID uuid.UUID) billingdomain.TenantBilling {
	t.Helper()
	item, ok, err := repo.GetTenantBilling(context.Background(), orgID)
	if err != nil {
		t.Fatalf("GetTenantBilling: %v", err)
	}
	if !ok {
		t.Fatalf("tenant settings not found")
	}
	return item
}

func strPtr(v string) *string { return &v }

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
