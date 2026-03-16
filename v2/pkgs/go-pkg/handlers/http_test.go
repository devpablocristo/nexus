package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterHealthEndpointsHealthzAlwaysOK(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterHealthEndpoints(mux, func(context.Context) error {
		return context.DeadlineExceeded
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("unexpected status: got=%d want=%d", got, want)
	}
}

func TestRegisterHealthEndpointsReadyzReflectsReadiness(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterHealthEndpoints(mux, func(context.Context) error {
		return context.DeadlineExceeded
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("unexpected status: got=%d want=%d", got, want)
	}
}

func TestComposeReadinessChecks(t *testing.T) {
	t.Parallel()

	check := ComposeReadinessChecks(
		nil,
		func(context.Context) error { return nil },
		func(context.Context) error { return context.Canceled },
	)

	if err := check(context.Background()); err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}
