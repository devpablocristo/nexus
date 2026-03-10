package operatorproxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"nexus/pkg/leaseauth"
)

func TestForwardEventsWithInternalKey(t *testing.T) {
	t.Setenv("NEXUS_AI_OPERATORS_INTERNAL_KEY", "op-key")
	t.Setenv("NEXUS_OPERATOR_API_KEY", "saas-api")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/events", r.URL.Path)
		require.Equal(t, "cursor=10&limit=2", r.URL.RawQuery)
		require.Equal(t, "saas-api", r.Header.Get("X-NEXUS-CORE-KEY"))
		require.Equal(t, "audit:read,admin:console:read", r.Header.Get("X-NEXUS-SCOPES"))
		require.Equal(t, "operator/observer", r.Header.Get("X-NEXUS-ACTOR"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"next_cursor":10}`))
	}))
	defer upstream.Close()
	t.Setenv("NEXUS_SAAS_URL", upstream.URL)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlerFromEnv()
	h.Register(r)

	req := httptest.NewRequest(http.MethodGet, "/internal/operators/events?cursor=10&limit=2", nil)
	req.Header.Set(headerOperatorKey, "op-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"items":[],"next_cursor":10}`, rec.Body.String())
}

func TestRejectsMissingInternalKey(t *testing.T) {
	t.Setenv("NEXUS_AI_OPERATORS_INTERNAL_KEY", "op-key")
	t.Setenv("NEXUS_OPERATOR_API_KEY", "saas-api")
	t.Setenv("NEXUS_SAAS_URL", "http://example.invalid")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlerFromEnv()
	h.Register(r)

	req := httptest.NewRequest(http.MethodGet, "/internal/operators/events?cursor=1&limit=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAppendEventWithInternalKey(t *testing.T) {
	t.Setenv("NEXUS_AI_OPERATORS_INTERNAL_KEY", "op-key")
	t.Setenv("NEXUS_SAAS_INTERNAL_KEY", "saas-internal")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/events", r.URL.Path)
		require.Equal(t, "saas-internal", r.Header.Get("X-NEXUS-SAAS-KEY"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	t.Setenv("NEXUS_SAAS_URL", upstream.URL)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlerFromEnv()
	h.Register(r)

	req := httptest.NewRequest(http.MethodPost, "/internal/operators/events/append", strings.NewReader(`{"org_id":"11111111-1111-1111-1111-111111111111","event_type":"incident.opened","payload":{"foo":"bar"}}`))
	req.Header.Set(headerOperatorKey, "op-key")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"ok":true}`, rec.Body.String())
}

func TestForwardActionWithExecutionLeaseHeaders(t *testing.T) {
	t.Setenv("NEXUS_AI_OPERATORS_INTERNAL_KEY", "op-key")
	t.Setenv("NEXUS_OPERATOR_API_KEY", "saas-api")
	t.Setenv("NEXUS_EXECUTION_LEASE_TOKEN_ISSUER", "nexus-core-test")
	t.Setenv("NEXUS_EXECUTION_LEASE_SIGNING_KEY", "lease-secret")

	token, err := leaseauth.SignToken("lease-secret", leaseauth.Claims{
		OrgID:          "org-1",
		LeaseID:        "lease-1",
		IntentID:       "intent-1",
		ToolName:       "operator-action",
		RiskClass:      "mutate_prod",
		CredentialMode: "aws_sts",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "nexus-core-test",
			Subject:   "execution_lease",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(5 * time.Minute)),
		},
	})
	require.NoError(t, err)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/actions/apply", r.URL.Path)
		require.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		require.Equal(t, token, r.Header.Get("X-Nexus-Execution-Token"))
		require.Equal(t, "lease-1", r.Header.Get("X-Nexus-Lease-Id"))
		require.Equal(t, "intent-1", r.Header.Get("X-Nexus-Intent-Id"))
		require.Equal(t, "aws_sts", r.Header.Get("X-Nexus-Credential-Mode"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	t.Setenv("NEXUS_SAAS_URL", upstream.URL)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlerFromEnv()
	h.Register(r)

	req := httptest.NewRequest(http.MethodPost, "/internal/operators/actions/apply", bytes.NewReader([]byte(`{"foo":"bar"}`)))
	req.Header.Set(headerOperatorKey, "op-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Nexus-Execution-Token", token)
	req.Header.Set("X-Nexus-Lease-Id", "lease-1")
	req.Header.Set("X-Nexus-Intent-Id", "intent-1")
	req.Header.Set("X-Nexus-Credential-Mode", "aws_sts")
	req.Header.Set("X-Nexus-Tool-Name", "operator-action")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"ok":true}`, rec.Body.String())
}

func TestRejectsInvalidExecutionLeaseToken(t *testing.T) {
	t.Setenv("NEXUS_AI_OPERATORS_INTERNAL_KEY", "op-key")
	t.Setenv("NEXUS_OPERATOR_API_KEY", "saas-api")
	t.Setenv("NEXUS_SAAS_URL", "http://example.invalid")
	t.Setenv("NEXUS_EXECUTION_LEASE_TOKEN_ISSUER", "nexus-core-test")
	t.Setenv("NEXUS_EXECUTION_LEASE_SIGNING_KEY", "lease-secret")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlerFromEnv()
	h.Register(r)

	req := httptest.NewRequest(http.MethodPost, "/internal/operators/actions/apply", bytes.NewReader([]byte(`{"foo":"bar"}`)))
	req.Header.Set(headerOperatorKey, "op-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")
	req.Header.Set("X-Nexus-Credential-Mode", "aws_sts")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
