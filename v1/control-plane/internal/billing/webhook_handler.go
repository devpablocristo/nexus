package billing

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	saasmetrics "control-plane/internal/shared/metrics"
)

const (
	maxStripeWebhookBodyBytes = 2 * 1024 * 1024
	stripeWebhookRateLimit    = 120
	stripeWebhookRateWindow   = 1 * time.Minute
)

var (
	stripeWHMu    sync.Mutex
	stripeWHCount int
	stripeWHReset time.Time
)

func checkStripeWebhookRateLimit() bool {
	stripeWHMu.Lock()
	defer stripeWHMu.Unlock()
	now := time.Now()
	if now.After(stripeWHReset) {
		stripeWHCount = 0
		stripeWHReset = now.Add(stripeWebhookRateWindow)
	}
	stripeWHCount++
	return stripeWHCount <= stripeWebhookRateLimit
}

func (h *Handler) RegisterWebhook(r *gin.Engine) {
	r.POST("/v1/webhooks/stripe", h.handleStripeWebhook)
}

func (h *Handler) handleStripeWebhook(c *gin.Context) {
	if !h.uc.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "INTERNAL", "message": "stripe billing is not configured"},
		})
		return
	}
	if !checkStripeWebhookRateLimit() {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": gin.H{"code": "RATE_LIMIT", "message": "stripe webhook rate limit exceeded"},
		})
		return
	}
	secret := h.uc.WebhookSecret()
	if secret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "INTERNAL", "message": "stripe webhook secret is not configured"},
		})
		return
	}
	payload, err := io.ReadAll(io.LimitReader(c.Request.Body, maxStripeWebhookBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid webhook payload"},
		})
		return
	}
	event, err := h.uc.stripe.ConstructWebhookEvent(payload, c.GetHeader("Stripe-Signature"), secret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "UNAUTHORIZED", "message": "invalid stripe signature"},
		})
		return
	}
	saasmetrics.WebhooksReceived.WithLabelValues("stripe", strings.TrimSpace(string(event.Type))).Inc()
	if err := h.uc.HandleWebhookEvent(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "INTERNAL", "message": "failed processing stripe webhook"},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
