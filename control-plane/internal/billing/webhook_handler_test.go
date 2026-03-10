package billing

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
)

func TestWebhookHandler_DisabledReturns503(t *testing.T) {
	h := NewHandler(&Usecases{
		stripeEnabled: false,
		stripe:        &fakeStripeClient{},
	})
	w := performWebhookRequest(t, h, `{"ok":true}`, "sig")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWebhookHandler_MissingSecretReturns503(t *testing.T) {
	h := NewHandler(&Usecases{
		stripeEnabled: true,
		webhookSecret: "",
		stripe:        &fakeStripeClient{},
	})
	w := performWebhookRequest(t, h, `{"ok":true}`, "sig")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWebhookHandler_InvalidSignatureReturns401(t *testing.T) {
	h := NewHandler(&Usecases{
		stripeEnabled: true,
		webhookSecret: "whsec_test",
		stripe: &fakeStripeClient{
			constructWebhookEventFn: func(payload []byte, sigHeader, secret string) (stripe.Event, error) {
				return stripe.Event{}, errors.New("invalid signature")
			},
		},
	})
	w := performWebhookRequest(t, h, `{"ok":true}`, "sig")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWebhookHandler_ValidEventReturns200(t *testing.T) {
	h := NewHandler(&Usecases{
		stripeEnabled: true,
		webhookSecret: "whsec_test",
		stripe: &fakeStripeClient{
			constructWebhookEventFn: func(payload []byte, sigHeader, secret string) (stripe.Event, error) {
				return stripe.Event{
					Type: stripe.EventType("unknown.event"),
					Data: &stripe.EventData{Raw: payload},
				}, nil
			},
		},
	})
	w := performWebhookRequest(t, h, `{"type":"unknown.event","data":{"object":{}}}`, "sig")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("expected ok response body, got %s", w.Body.String())
	}
}

func performWebhookRequest(t *testing.T, h *Handler, body, signature string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterWebhook(r)

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signature)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
