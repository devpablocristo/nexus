package billing

import (
	"errors"
	"strings"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/client"
	"github.com/stripe/stripe-go/v81/webhook"
)

type StripeClientPort interface {
	CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error)
	CreateCheckoutSession(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
	GetSubscription(subscriptionID string) (*stripe.Subscription, error)
	ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error)
}

// StripeClient wraps the Stripe Go SDK using client.API (per-instance backends)
// instead of the global stripe.Key, making it safe for concurrent use.
type StripeClient struct {
	api *client.API
}

func NewStripeClient(secretKey string) *StripeClient {
	secretKey = strings.TrimSpace(secretKey)
	sc := &StripeClient{}
	if secretKey != "" {
		sc.api = &client.API{}
		sc.api.Init(secretKey, nil)
	}
	return sc
}

func (c *StripeClient) CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error) {
	if err := c.ensureConfigured(); err != nil {
		return nil, err
	}
	return c.api.Customers.New(params)
}

func (c *StripeClient) CreateCheckoutSession(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	if err := c.ensureConfigured(); err != nil {
		return nil, err
	}
	return c.api.CheckoutSessions.New(params)
}

func (c *StripeClient) CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	if err := c.ensureConfigured(); err != nil {
		return nil, err
	}
	return c.api.BillingPortalSessions.New(params)
}

func (c *StripeClient) GetSubscription(subscriptionID string) (*stripe.Subscription, error) {
	if err := c.ensureConfigured(); err != nil {
		return nil, err
	}
	return c.api.Subscriptions.Get(subscriptionID, nil)
}

func (c *StripeClient) ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, sigHeader, secret)
}

func (c *StripeClient) ensureConfigured() error {
	if c.api == nil {
		return errors.New("stripe client not configured")
	}
	return nil
}
