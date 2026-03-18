package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityMiddlewareSetsHeaders(t *testing.T) {
	t.Parallel()

	handler := SecurityMiddleware(SecurityConfig{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/resources", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("unexpected nosniff header: %q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("unexpected frame options header: %q", got)
	}
}

func TestSecurityMiddlewareHandlesAllowedCORSPreflight(t *testing.T) {
	t.Parallel()

	handler := SecurityMiddleware(SecurityConfig{
		AllowedOrigins: []string{"https://console.example.com"},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/resources", nil)
	req.Header.Set("Origin", "https://console.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("unexpected status: got=%d want=%d", got, want)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://console.example.com" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
}

func TestSecurityMiddlewareRejectsUnknownCORSPreflight(t *testing.T) {
	t.Parallel()

	handler := SecurityMiddleware(SecurityConfig{
		AllowedOrigins: []string{"https://console.example.com"},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/resources", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusForbidden; got != want {
		t.Fatalf("unexpected status: got=%d want=%d", got, want)
	}
}

func TestSecurityMiddlewareSetsHSTSForHTTPS(t *testing.T) {
	t.Parallel()

	handler := SecurityMiddleware(SecurityConfig{
		HSTSMaxAge: "31536000",
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/resources", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("unexpected hsts header: %q", got)
	}
}
