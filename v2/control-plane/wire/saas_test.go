package wire

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
)

func TestWrapAuthBypassesPublicSaaSRoutes(t *testing.T) {
	t.Parallel()

	nexusAuth, err := sharedapikey.NewAuthenticator("nexus=secret")
	if err != nil {
		t.Fatalf("unexpected auth setup error: %v", err)
	}

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapAuth(base, nexusAuth, &SaaSServices{
		AuthMiddleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-SaaS-Auth", "used")
				next.ServeHTTP(w, r)
			})
		},
		MeteringMW: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-SaaS-Metering", "used")
				next.ServeHTTP(w, r)
			})
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("X-SaaS-Auth"); got != "" {
		t.Fatalf("expected public route to bypass saas auth, got %q", got)
	}
}

func TestWrapAuthUsesSaaSMiddlewareOnProtectedSaaSRoutes(t *testing.T) {
	t.Parallel()

	nexusAuth, err := sharedapikey.NewAuthenticator("nexus=secret")
	if err != nil {
		t.Fatalf("unexpected auth setup error: %v", err)
	}

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapAuth(base, nexusAuth, &SaaSServices{
		AuthMiddleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-SaaS-Auth", "used")
				next.ServeHTTP(w, r)
			})
		},
		MeteringMW: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-SaaS-Metering", "used")
				next.ServeHTTP(w, r)
			})
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/billing/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("X-SaaS-Auth"); got != "used" {
		t.Fatalf("expected protected saas route to use saas auth, got %q", got)
	}
	if got := rec.Header().Get("X-SaaS-Metering"); got != "used" {
		t.Fatalf("expected protected saas route to use metering, got %q", got)
	}
}

func TestWrapAuthUsesNexusAuthOnCoreRoutes(t *testing.T) {
	t.Parallel()

	nexusAuth, err := sharedapikey.NewAuthenticator("nexus=secret")
	if err != nil {
		t.Fatalf("unexpected auth setup error: %v", err)
	}

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapAuth(base, nexusAuth, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/resources", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d without nexus api key, got %d", http.StatusUnauthorized, rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/resources", nil)
	req.Header.Set(sharedapikey.HeaderName, "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d with nexus api key, got %d", http.StatusOK, rec.Code)
	}
}
