package billing

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"

	"nexus/pkg/types"
)

func TestHandler_BillingEndpointsSuccess(t *testing.T) {
	db := newBillingTestDB(t)
	repo := NewRepository(db)
	port := &fakeTenantSettingsPort{db: db}
	orgID := uuid.New()
	seedOrg(t, db, orgID, "Globex")
	seedTenantSettings(t, db, orgID, tenantSeed{
		PlanCode:       "growth",
		BillingStatus:  "active",
		CustomerID:     strPtr("cus_seed"),
		SubscriptionID: strPtr("sub_seed"),
		HardLimits: map[string]any{
			"tools_max": 75, "run_rpm": 1200, "audit_retention_days": 90,
		},
	})

	uc := &Usecases{
		repo:           repo,
		tenantSettings: port,
		stripe: &fakeStripeClient{
			createCheckoutSessionFn: func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
				return &stripe.CheckoutSession{URL: "https://stripe.test/checkout"}, nil
			},
			createPortalSessionFn: func(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
				return &stripe.BillingPortalSession{URL: "https://stripe.test/portal"}, nil
			},
			getSubscriptionFn: func(subscriptionID string) (*stripe.Subscription, error) {
				return &stripe.Subscription{CurrentPeriodEnd: time.Now().UTC().Add(24 * time.Hour).Unix()}, nil
			},
		},
		stripeEnabled:   true,
		priceStarter:    "price_starter",
		priceGrowth:     "price_growth",
		priceEnterprise: "price_enterprise",
		towerBaseURL:    "http://localhost:5173",
	}
	h := NewHandler(uc)
	r := newAuthedBillingRouter(orgID, "secops", nil, h)

	assertHTTPStatus(t, r, http.MethodGet, "/v1/billing/status", nil, http.StatusOK)
	assertHTTPStatus(t, r, http.MethodGet, "/v1/billing/usage", nil, http.StatusOK)
	assertHTTPStatus(t, r, http.MethodPost, "/v1/billing/checkout", []byte(`{"plan_code":"growth"}`), http.StatusOK)
	assertHTTPStatus(t, r, http.MethodPost, "/v1/billing/portal", []byte(`{"return_url":"http://localhost:5173/billing"}`), http.StatusOK)
}

func TestHandler_CheckoutRequiresWritePermission(t *testing.T) {
	db := newBillingTestDB(t)
	repo := NewRepository(db)
	port := &fakeTenantSettingsPort{db: db}
	orgID := uuid.New()
	seedOrg(t, db, orgID, "NoWrite")
	seedTenantSettings(t, db, orgID, tenantSeed{
		PlanCode:      "starter",
		BillingStatus: "trialing",
		HardLimits: map[string]any{
			"tools_max": 20, "run_rpm": 300, "audit_retention_days": 30,
		},
	})

	uc := &Usecases{
		repo:           repo,
		tenantSettings: port,
		stripe:         &fakeStripeClient{},
		stripeEnabled:  true,
		priceGrowth:    "price_growth",
	}
	h := NewHandler(uc)
	r := newAuthedBillingRouter(orgID, "viewer", []string{"admin:console:read"}, h)

	assertHTTPStatus(t, r, http.MethodPost, "/v1/billing/checkout", []byte(`{"plan_code":"growth"}`), http.StatusForbidden)
}

func newAuthedBillingRouter(orgID uuid.UUID, role string, scopes []string, h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), orgID)
		c.Set(string(types.CtxKeyRole), role)
		c.Set(string(types.CtxKeyScopes), scopes)
		c.Set(string(types.CtxKeyActor), "user_test")
		c.Next()
	})
	h.Register(v1)
	return r
}

func assertHTTPStatus(t *testing.T, r *gin.Engine, method, path string, body []byte, expected int) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != expected {
		t.Fatalf("%s %s expected %d, got %d body=%s", method, path, expected, w.Code, w.Body.String())
	}
}
