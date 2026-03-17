package wire

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	saasctxkeys "github.com/devpablocristo/saas-core/shared/ctxkeys"
	saashttperr "github.com/devpablocristo/saas-core/shared/httperr"
	saasmiddleware "github.com/devpablocristo/saas-core/shared/middleware"
	"github.com/google/uuid"
)

type testPrincipalVerifier struct {
	principal saasmiddleware.Principal
	err       error
}

func (v testPrincipalVerifier) Verify(context.Context, string) (saasmiddleware.Principal, error) {
	return v.principal, v.err
}

func TestWrapAuthHTTPPublicRoutesBypassNexusAndSaaSAuth(t *testing.T) {
	t.Parallel()

	handler := newWrappedSaaSTestHandler(t, testWrappedHandlerConfig{})

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "org bootstrap", method: http.MethodPost, path: "/orgs"},
		{name: "clerk webhook", method: http.MethodPost, path: "/webhooks/clerk"},
		{name: "stripe webhook", method: http.MethodPost, path: "/v1/webhooks/stripe"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if got := rec.Header().Get("X-SaaS-Auth"); got != "" {
				t.Fatalf("expected public route to bypass saas auth, got %q", got)
			}
		})
	}
}

func TestWrapAuthHTTPBillingStatusRequiresSaaSAuth(t *testing.T) {
	t.Parallel()

	handler := newWrappedSaaSTestHandler(t, testWrappedHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/billing/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d without saas credentials, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestWrapAuthHTTPBillingStatusRejectsNexusAPIKeyOnly(t *testing.T) {
	t.Parallel()

	handler := newWrappedSaaSTestHandler(t, testWrappedHandlerConfig{
		apiKeyErr: saashttperr.New(http.StatusUnauthorized, saashttperr.CodeUnauthorized, "invalid api key"),
	})

	req := httptest.NewRequest(http.MethodGet, "/billing/status", nil)
	req.Header.Set(sharedapikey.HeaderName, "nexus-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d with only nexus api key, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestWrapAuthHTTPBillingStatusInjectsJWTContext(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	handler := newWrappedSaaSTestHandler(t, testWrappedHandlerConfig{
		jwtPrincipal: saasmiddleware.Principal{
			OrgID:      orgID.String(),
			Actor:      "alice@example.com",
			Role:       "admin",
			Scopes:     []string{"admin:console:read"},
			AuthMethod: "jwt",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/billing/status", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("X-SaaS-Auth"); got != "jwt" {
		t.Fatalf("expected jwt auth header marker, got %q", got)
	}
	if got := rec.Header().Get("X-SaaS-Metering"); got != "used" {
		t.Fatalf("expected metering marker, got %q", got)
	}
	if got := rec.Header().Get("X-Org-ID"); got != orgID.String() {
		t.Fatalf("expected org context %q, got %q", orgID.String(), got)
	}
	if got := rec.Header().Get("X-Actor"); got != "alice@example.com" {
		t.Fatalf("expected actor context, got %q", got)
	}
	if got := rec.Header().Get("X-Role"); got != "admin" {
		t.Fatalf("expected role context, got %q", got)
	}
}

func TestWrapAuthHTTPBillingStatusInjectsAPIKeyContext(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	handler := newWrappedSaaSTestHandler(t, testWrappedHandlerConfig{
		apiKeyPrincipal: saasmiddleware.Principal{
			OrgID:      orgID.String(),
			Scopes:     []string{"admin:console:write"},
			AuthMethod: "api_key",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/billing/status", nil)
	req.Header.Set("X-API-Key", "tenant-api-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("X-SaaS-Auth"); got != "api_key" {
		t.Fatalf("expected api_key auth header marker, got %q", got)
	}
	if got := rec.Header().Get("X-Org-ID"); got != orgID.String() {
		t.Fatalf("expected org context %q, got %q", orgID.String(), got)
	}
}

type testWrappedHandlerConfig struct {
	jwtPrincipal    saasmiddleware.Principal
	apiKeyPrincipal saasmiddleware.Principal
	jwtErr          error
	apiKeyErr       error
}

func newWrappedSaaSTestHandler(t *testing.T, cfg testWrappedHandlerConfig) http.Handler {
	t.Helper()

	nexusAuth, err := sharedapikey.NewAuthenticator("nexus=nexus-secret")
	if err != nil {
		t.Fatalf("unexpected nexus auth setup error: %v", err)
	}

	if cfg.jwtPrincipal.OrgID == "" {
		cfg.jwtPrincipal = saasmiddleware.Principal{
			OrgID:      uuid.NewString(),
			Actor:      "alice@example.com",
			Role:       "admin",
			Scopes:     []string{"admin:console:read"},
			AuthMethod: "jwt",
		}
	}
	if cfg.apiKeyPrincipal.OrgID == "" {
		cfg.apiKeyPrincipal = saasmiddleware.Principal{
			OrgID:      uuid.NewString(),
			Scopes:     []string{"admin:console:write"},
			AuthMethod: "api_key",
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orgs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /webhooks/clerk", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /v1/webhooks/stripe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /billing/status", func(w http.ResponseWriter, r *http.Request) {
		if authMethod, _ := r.Context().Value(saasctxkeys.AuthMethod).(string); authMethod != "" {
			w.Header().Set("X-SaaS-Auth", authMethod)
		}
		if orgID, ok := r.Context().Value(saasctxkeys.OrgID).(uuid.UUID); ok {
			w.Header().Set("X-Org-ID", orgID.String())
		}
		if actor, _ := r.Context().Value(saasctxkeys.Actor).(string); actor != "" {
			w.Header().Set("X-Actor", actor)
		}
		if role, _ := r.Context().Value(saasctxkeys.Role).(string); role != "" {
			w.Header().Set("X-Role", role)
		}
		w.WriteHeader(http.StatusOK)
	})

	svc := &SaaSServices{
		AuthMiddleware: saasmiddleware.NewAuthMiddleware(
			testPrincipalVerifier{principal: cfg.jwtPrincipal, err: cfg.jwtErr},
			testPrincipalVerifier{principal: cfg.apiKeyPrincipal, err: cfg.apiKeyErr},
		),
		MeteringMW: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-SaaS-Metering", "used")
				next.ServeHTTP(w, r)
			})
		},
	}

	return WrapAuth(mux, nexusAuth, svc)
}
