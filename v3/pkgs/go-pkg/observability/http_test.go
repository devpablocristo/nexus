package observability

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestMiddlewareSetsRequestIDAndLogsJSON(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := NewJSONLoggerWriter("svc-test", &logs)

	mux := http.NewServeMux()
	mux.Handle("POST /v1/resources", Middleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, ok := RequestIDFromContext(r.Context())
		if !ok || requestID == "" {
			t.Fatal("expected request id in context")
		}
		LoggerFromContext(r.Context()).Info("inside handler")
		w.WriteHeader(http.StatusCreated)
	})))

	req := httptest.NewRequest(http.MethodPost, "/v1/resources", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got == "" {
		t.Fatal("expected request id response header")
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least two log lines, got=%d body=%q", len(lines), logs.String())
	}

	var access map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &access); err != nil {
		t.Fatalf("unmarshal access log: %v", err)
	}
	if access["service"] != "svc-test" {
		t.Fatalf("unexpected service: %#v", access)
	}
	if access["event"] != "http_request_completed" {
		t.Fatalf("unexpected event: %#v", access)
	}
	if access["status"] != float64(http.StatusCreated) {
		t.Fatalf("unexpected status: %#v", access)
	}
	if access["route"] != "/v1/resources" {
		t.Fatalf("unexpected route: %#v", access)
	}
	if access["request_id"] == "" {
		t.Fatalf("missing request id: %#v", access)
	}
	if got := strings.Count(lines[len(lines)-1], "\"request_id\""); got != 1 {
		t.Fatalf("expected one request_id field in access log, got=%d line=%q", got, lines[len(lines)-1])
	}
	if got := strings.Count(lines[len(lines)-1], "\"method\""); got != 1 {
		t.Fatalf("expected one method field in access log, got=%d line=%q", got, lines[len(lines)-1])
	}
	if got := strings.Count(lines[len(lines)-1], "\"path\""); got != 1 {
		t.Fatalf("expected one path field in access log, got=%d line=%q", got, lines[len(lines)-1])
	}
}

func TestMiddlewarePreservesIncomingRequestID(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := NewJSONLoggerWriter("svc-test", &logs)

	mux := http.NewServeMux()
	mux.Handle("GET /v1/resources", Middleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, ok := RequestIDFromContext(r.Context())
		if !ok || requestID != "req-123" {
			t.Fatalf("unexpected request id: got=%q ok=%v", requestID, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodGet, "/v1/resources", nil)
	req.Header.Set(RequestIDHeader, "req-123")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got != "req-123" {
		t.Fatalf("unexpected response request id: %q", got)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")

	var access map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &access); err != nil {
		t.Fatalf("unmarshal access log: %v", err)
	}
	if access["request_id"] != "req-123" {
		t.Fatalf("unexpected request id in access log: %#v", access)
	}
	if access["route"] != "/v1/resources" {
		t.Fatalf("unexpected route in access log: %#v", access)
	}
	if got := strings.Count(lines[len(lines)-1], strconv.Quote("request_id")); got != 1 {
		t.Fatalf("expected one request_id field in access log, got=%d line=%q", got, lines[len(lines)-1])
	}
}

func TestApplyRequestID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	ctx := ContextWithRequestID(req.Context(), "req-xyz")

	ApplyRequestID(req, ctx)

	if got := req.Header.Get(RequestIDHeader); got != "req-xyz" {
		t.Fatalf("unexpected request id header: %q", got)
	}
}
