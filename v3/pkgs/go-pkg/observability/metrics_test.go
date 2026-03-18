package observability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMiddlewareWithMetricsRecordsREDMetrics(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics()
	handler := MiddlewareWithMetrics(NewJSONLoggerWriter("svc-test", nil), metrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Pattern, "GET /v1/resources/{id}"; got != want {
			t.Fatalf("unexpected pattern in handler: got=%q want=%q", got, want)
		}
		w.WriteHeader(http.StatusCreated)
	}))

	mux := http.NewServeMux()
	mux.Handle("GET /v1/resources/{id}", handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/resources/123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if got := testutil.ToFloat64(metrics.httpRequests.WithLabelValues(http.MethodGet, "/v1/resources/{id}", "201")); got != 1 {
		t.Fatalf("unexpected request counter: %v", got)
	}
	if got := testutil.ToFloat64(metrics.httpErrors.WithLabelValues(http.MethodGet, "/v1/resources/{id}", "201")); got != 0 {
		t.Fatalf("unexpected error counter: %v", got)
	}
	metricsRec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsRec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, err := io.ReadAll(metricsRec.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	if !strings.Contains(string(body), `nexus_http_request_duration_seconds_bucket{method="GET",route="/v1/resources/{id}"`) {
		t.Fatalf("expected duration metric in scrape output, got=%q", string(body))
	}
}

func TestMiddlewareWithMetricsTracksErrorsAndMetricsEndpoint(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics()
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	wrapped := WithMetricsEndpoint(base, metrics.Handler())
	handler := MiddlewareWithMetrics(NewJSONLoggerWriter("svc-test", nil), metrics, wrapped)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected /metrics status: %d", rec.Code)
	}
	if got := testutil.ToFloat64(metrics.httpRequests.WithLabelValues(http.MethodGet, "/metrics", "200")); got != 1 {
		t.Fatalf("unexpected /metrics request counter: %v", got)
	}

	errReq := httptest.NewRequest(http.MethodPost, "/missing", nil)
	errRec := httptest.NewRecorder()
	handler.ServeHTTP(errRec, errReq)

	if got := testutil.ToFloat64(metrics.httpErrors.WithLabelValues(http.MethodPost, "unmatched", "401")); got != 1 {
		t.Fatalf("unexpected error counter: %v", got)
	}
}

func TestBusinessCounters(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics()

	metrics.IncActionCreated("withdrawal")
	metrics.IncActionBlocked("withdrawal")
	metrics.IncActionApproved("withdrawal")
	metrics.IncActionExecuted("withdrawal")
	metrics.IncIncidentCreated("blocked_action", "high")
	metrics.IncAlertCreated("slack", "high")

	if got := testutil.ToFloat64(metrics.actionEvents.WithLabelValues("created", "withdrawal")); got != 1 {
		t.Fatalf("unexpected actions created counter: %v", got)
	}
	if got := testutil.ToFloat64(metrics.actionEvents.WithLabelValues("blocked", "withdrawal")); got != 1 {
		t.Fatalf("unexpected actions blocked counter: %v", got)
	}
	if got := testutil.ToFloat64(metrics.actionEvents.WithLabelValues("approved", "withdrawal")); got != 1 {
		t.Fatalf("unexpected actions approved counter: %v", got)
	}
	if got := testutil.ToFloat64(metrics.actionEvents.WithLabelValues("executed", "withdrawal")); got != 1 {
		t.Fatalf("unexpected actions executed counter: %v", got)
	}
	if got := testutil.ToFloat64(metrics.incidentCreated.WithLabelValues("blocked_action", "high")); got != 1 {
		t.Fatalf("unexpected incidents counter: %v", got)
	}
	if got := testutil.ToFloat64(metrics.alertCreated.WithLabelValues("slack", "high")); got != 1 {
		t.Fatalf("unexpected alerts counter: %v", got)
	}
}
