package apikey

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAuthenticatorRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := NewAuthenticator("admin")
	if err == nil {
		t.Fatal("expected invalid config error")
	}
}

func TestMiddlewareAuthorizesHeaderAndStoresPrincipal(t *testing.T) {
	t.Parallel()

	authn, err := NewAuthenticator("admin=secret")
	if err != nil {
		t.Fatalf("NewAuthenticator returned error: %v", err)
	}

	handler := authn.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("expected principal in context")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"principal": principal.Name})
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/resources", nil)
	req.Header.Set(HeaderName, "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["principal"] != "admin" {
		t.Fatalf("unexpected principal: %#v", payload)
	}
}

func TestMiddlewareAllowsHealthWithoutAuth(t *testing.T) {
	t.Parallel()

	authn, err := NewAuthenticator("admin=secret")
	if err != nil {
		t.Fatalf("NewAuthenticator returned error: %v", err)
	}

	handler := authn.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
	}
}

func TestMiddlewareAllowsReadyWithoutAuth(t *testing.T) {
	t.Parallel()

	authn, err := NewAuthenticator("admin=secret")
	if err != nil {
		t.Fatalf("NewAuthenticator returned error: %v", err)
	}

	handler := authn.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
	}
}

func TestMiddlewareRejectsUnauthorized(t *testing.T) {
	t.Parallel()

	authn, err := NewAuthenticator("admin=secret")
	if err != nil {
		t.Fatalf("NewAuthenticator returned error: %v", err)
	}

	handler := authn.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/resources", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsMetricsWithoutAuth(t *testing.T) {
	t.Parallel()

	authn, err := NewAuthenticator("admin=secret")
	if err != nil {
		t.Fatalf("NewAuthenticator returned error: %v", err)
	}

	handler := authn.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsMalformedAuthorizationWithoutLeaking(t *testing.T) {
	t.Parallel()

	authn, err := NewAuthenticator("admin=secret")
	if err != nil {
		t.Fatalf("NewAuthenticator returned error: %v", err)
	}

	handler := authn.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/resources", nil)
	req.Header.Set("Authorization", "Token secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); body == "" || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("expected json body, got %q", body)
	}
	if got, want := rec.Body.String(), "{\"error\":{\"code\":\"UNAUTHORIZED\",\"message\":\"valid api key required\"}}\n"; got != want {
		t.Fatalf("unexpected body: %q", got)
	}
}
