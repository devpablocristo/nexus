package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	WebhooksReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexus_saas_webhooks_received_total",
			Help: "Total number of webhooks received by source and event type.",
		},
		[]string{"source", "event_type"},
	)

	BillingCheckouts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexus_saas_billing_checkouts_total",
			Help: "Total number of Stripe checkout sessions created by plan code.",
		},
		[]string{"plan_code"},
	)

	NotificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexus_saas_notifications_sent_total",
			Help: "Total number of sent notifications by notification type and channel.",
		},
		[]string{"notification_type", "channel"},
	)

	AlertsEvaluated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "nexus_saas_alerts_evaluated_total",
			Help: "Total number of alert evaluation cycles executed.",
		},
	)

	AlertsFired = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexus_saas_alerts_fired_total",
			Help: "Total number of fired alerts by rule name.",
		},
		[]string{"rule_name"},
	)

	UsageMeteringEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nexus_saas_usage_events_total",
			Help: "Total number of usage metering events by org and counter.",
		},
		[]string{"org_id", "counter"},
	)
)

func init() {
	// Pre-register baseline series so dashboards/grep can discover metrics
	// before the first real business event arrives.
	WebhooksReceived.WithLabelValues("clerk", "unknown").Add(0)
	WebhooksReceived.WithLabelValues("stripe", "unknown").Add(0)

	BillingCheckouts.WithLabelValues("starter").Add(0)
	BillingCheckouts.WithLabelValues("growth").Add(0)
	BillingCheckouts.WithLabelValues("enterprise").Add(0)

	NotificationsSent.WithLabelValues("welcome", "email").Add(0)
	NotificationsSent.WithLabelValues("plan_upgraded", "email").Add(0)
	NotificationsSent.WithLabelValues("payment_failed", "email").Add(0)
	NotificationsSent.WithLabelValues("subscription_canceled", "email").Add(0)
	NotificationsSent.WithLabelValues("incident_opened", "email").Add(0)
	NotificationsSent.WithLabelValues("incident_closed", "email").Add(0)

	AlertsFired.WithLabelValues("none").Add(0)
}
