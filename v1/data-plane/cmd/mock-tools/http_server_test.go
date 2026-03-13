package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"data-plane/internal/shared/leasehttp"
	"nexus/pkg/leaseauth"
)

func mustExecutionToken(t *testing.T, secret, issuer string) string {
	t.Helper()
	token, err := leaseauth.SignToken(secret, leaseauth.Claims{
		OrgID:          "org-1",
		LeaseID:        "lease-1",
		IntentID:       "intent-1",
		ToolName:       "mock-echo",
		RiskClass:      "mutate_prod",
		CredentialMode: "aws_sts",
		Provider:       "aws",
		Scope:          "sts_assume_role",
		TargetEnv:      "prod",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "execution_lease",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestEchoAcceptsValidExecutionLeaseToken(t *testing.T) {
	auth, err := NewAuthArtifacts()
	if err != nil {
		t.Fatalf("auth artifacts: %v", err)
	}
	verifier := &leasehttp.Verifier{Issuer: "nexus-core-test", SigningKey: "lease-secret"}
	r := NewRouter(auth, verifier)

	body, _ := json.Marshal(map[string]any{"hello": "world"})
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mustExecutionToken(t, "lease-secret", "nexus-core-test"))
	req.Header.Set("X-Nexus-Credential-Mode", "aws_sts")
	req.Header.Set("X-Nexus-Lease-Id", "lease-1")
	req.Header.Set("X-Nexus-Intent-Id", "intent-1")
	req.Header.Set("X-Nexus-Tool-Name", "mock-echo")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, _ := resp["execution_lease_verified"].(bool); !got {
		t.Fatalf("expected verified execution lease")
	}
	if got, _ := resp["execution_lease_id"].(string); got != "lease-1" {
		t.Fatalf("expected lease-1 got %q", got)
	}
}

func TestEchoRejectsMissingExecutionLeaseToken(t *testing.T) {
	auth, err := NewAuthArtifacts()
	if err != nil {
		t.Fatalf("auth artifacts: %v", err)
	}
	verifier := &leasehttp.Verifier{Issuer: "nexus-core-test", SigningKey: "lease-secret"}
	r := NewRouter(auth, verifier)

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"hello":"world"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Nexus-Credential-Mode", "aws_sts")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEchoRejectsExecutionLeaseMismatch(t *testing.T) {
	auth, err := NewAuthArtifacts()
	if err != nil {
		t.Fatalf("auth artifacts: %v", err)
	}
	verifier := &leasehttp.Verifier{Issuer: "nexus-core-test", SigningKey: "lease-secret"}
	r := NewRouter(auth, verifier)

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"hello":"world"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mustExecutionToken(t, "lease-secret", "nexus-core-test"))
	req.Header.Set("X-Nexus-Credential-Mode", "aws_sts")
	req.Header.Set("X-Nexus-Lease-Id", "other-lease")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEchoAllowsRegularBearerAuthWithoutLeaseMetadata(t *testing.T) {
	auth, err := NewAuthArtifacts()
	if err != nil {
		t.Fatalf("auth artifacts: %v", err)
	}
	verifier := &leasehttp.Verifier{Issuer: "nexus-core-test", SigningKey: "lease-secret"}
	r := NewRouter(auth, verifier)

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"hello":"world"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer regular-api-token")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, _ := resp["auth_present"].(bool); !got {
		t.Fatalf("expected auth header to reach upstream")
	}
	if got, _ := resp["execution_lease_verified"].(bool); got {
		t.Fatalf("did not expect lease verification for regular bearer auth")
	}
}
