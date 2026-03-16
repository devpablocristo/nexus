package observability

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsRoute = "/metrics"

// Metrics exposes Prometheus collectors for service RED metrics and minimum business counters.
type Metrics struct {
	registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpErrors   *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec

	actionEvents    *prometheus.CounterVec
	incidentCreated *prometheus.CounterVec
	alertCreated    *prometheus.CounterVec
}

// NewMetrics builds a service-local registry with runtime and business metrics.
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()

	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_http_requests_total",
		Help: "Total HTTP requests handled by route and status code.",
	}, []string{"method", "route", "status_code"})
	httpErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_http_request_errors_total",
		Help: "Total HTTP requests that completed with an error status.",
	}, []string{"method", "route", "status_code"})
	httpDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nexus_http_request_duration_seconds",
		Help:    "HTTP request latency by route.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "route"})
	actionEvents := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_actions_total",
		Help: "Business action lifecycle events.",
	}, []string{"event", "action_type"})
	incidentCreated := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_incidents_created_total",
		Help: "Total incidents created by trigger and severity.",
	}, []string{"trigger", "severity"})
	alertCreated := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_alerts_created_total",
		Help: "Total alerts created by channel and severity.",
	}, []string{"channel", "severity"})

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpRequests,
		httpErrors,
		httpDuration,
		actionEvents,
		incidentCreated,
		alertCreated,
	)

	return &Metrics{
		registry:        registry,
		httpRequests:    httpRequests,
		httpErrors:      httpErrors,
		httpDuration:    httpDuration,
		actionEvents:    actionEvents,
		incidentCreated: incidentCreated,
		alertCreated:    alertCreated,
	}
}

// Handler returns the Prometheus scrape endpoint.
func (m *Metrics) Handler() http.Handler {
	if m == nil || m.registry == nil {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// WithMetricsEndpoint routes /metrics to the provided handler and delegates everything else.
func WithMetricsEndpoint(next http.Handler, metricsHandler http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	if metricsHandler == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == metricsRoute {
			metricsHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ObserveHTTPRequest records RED metrics for one completed request.
func (m *Metrics) ObserveHTTPRequest(r *http.Request, status int, duration time.Duration) {
	if m == nil || r == nil {
		return
	}
	route := routeLabel(r)
	statusCode := strconv.Itoa(status)
	m.httpRequests.WithLabelValues(r.Method, route, statusCode).Inc()
	if status >= http.StatusBadRequest {
		m.httpErrors.WithLabelValues(r.Method, route, statusCode).Inc()
	}
	m.httpDuration.WithLabelValues(r.Method, route).Observe(duration.Seconds())
}

// IncActionCreated records a created action.
func (m *Metrics) IncActionCreated(actionType string) {
	m.incActionEvent("created", actionType)
}

// IncActionBlocked records a blocked action.
func (m *Metrics) IncActionBlocked(actionType string) {
	m.incActionEvent("blocked", actionType)
}

// IncActionApproved records an approved action.
func (m *Metrics) IncActionApproved(actionType string) {
	m.incActionEvent("approved", actionType)
}

// IncActionExecuted records an executed action.
func (m *Metrics) IncActionExecuted(actionType string) {
	m.incActionEvent("executed", actionType)
}

// IncIncidentCreated records a created incident.
func (m *Metrics) IncIncidentCreated(trigger, severity string) {
	if m == nil {
		return
	}
	m.incidentCreated.WithLabelValues(normalizeMetricLabel(trigger), normalizeMetricLabel(severity)).Inc()
}

// IncAlertCreated records a created alert.
func (m *Metrics) IncAlertCreated(channel, severity string) {
	if m == nil {
		return
	}
	m.alertCreated.WithLabelValues(normalizeMetricLabel(channel), normalizeMetricLabel(severity)).Inc()
}

func (m *Metrics) incActionEvent(event, actionType string) {
	if m == nil {
		return
	}
	m.actionEvents.WithLabelValues(normalizeMetricLabel(event), normalizeMetricLabel(actionType)).Inc()
}

func routeLabel(r *http.Request) string {
	if r == nil {
		return "unmatched"
	}
	pattern := strings.TrimSpace(r.Pattern)
	if pattern == "" {
		if r.URL != nil && r.URL.Path == metricsRoute {
			return metricsRoute
		}
		return "unmatched"
	}
	if _, route, ok := strings.Cut(pattern, " "); ok {
		pattern = route
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "unmatched"
	}
	return pattern
}

func normalizeMetricLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
