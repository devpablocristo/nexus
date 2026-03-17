package wire

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestNewServerSaaSOrgBillingFlow(t *testing.T) {
	baseDatabaseURL := resolveSaaSTestDatabaseURL(t)
	databaseURL := provisionIsolatedSaaSSchema(t, baseDatabaseURL)

	handler, cleanup, err := NewServer(Config{
		NexusAPIKeys: "nexus=secret",
		SaaS: SaaSConfig{
			DatabaseURL:     databaseURL,
			StripeSecretKey: "sk_test_dummy",
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	defer cleanupAfterAsyncMetering(cleanup)

	createReqBody, err := json.Marshal(map[string]any{
		"name":   "acme-" + uuid.NewString(),
		"scopes": []string{"admin:console:read"},
	})
	if err != nil {
		t.Fatalf("marshal create org body: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/orgs", bytes.NewReader(createReqBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d from POST /orgs, got %d body=%s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var createResp struct {
		OrgID  string `json:"org_id"`
		APIKey string `json:"api_key"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create org response: %v", err)
	}
	if createResp.OrgID == "" || createResp.APIKey == "" {
		t.Fatalf("expected org_id and api_key in create response, got %#v", createResp)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/billing/status", nil)
	statusReq.Header.Set("X-API-Key", createResp.APIKey)
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected status %d from GET /billing/status, got %d body=%s", http.StatusOK, statusRec.Code, statusRec.Body.String())
	}

	var statusResp struct {
		PlanCode      string `json:"plan_code"`
		BillingStatus string `json:"billing_status"`
		HardLimits    struct {
			ToolsMax           int `json:"tools_max"`
			RunRPM             int `json:"run_rpm"`
			AuditRetentionDays int `json:"audit_retention_days"`
		} `json:"hard_limits"`
		Usage struct {
			Period string `json:"period"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode billing status response: %v", err)
	}
	if statusResp.PlanCode != "starter" {
		t.Fatalf("expected starter plan, got %#v", statusResp)
	}
	if statusResp.BillingStatus != "trialing" {
		t.Fatalf("expected trialing billing status, got %#v", statusResp)
	}
	if statusResp.HardLimits.ToolsMax != 20 || statusResp.HardLimits.RunRPM != 300 || statusResp.HardLimits.AuditRetentionDays != 30 {
		t.Fatalf("unexpected default hard limits: %#v", statusResp.HardLimits)
	}
	if strings.TrimSpace(statusResp.Usage.Period) == "" {
		t.Fatalf("expected usage period in billing response, got %#v", statusResp)
	}
}

func TestNewServerSaaSClerkOIDCJWTFlow(t *testing.T) {
	baseDatabaseURL := resolveSaaSTestDatabaseURL(t)
	databaseURL := provisionIsolatedSaaSSchema(t, baseDatabaseURL)

	oidc := newTestOIDCIssuer(t)
	clerkSecret := newTestSvixSecret(t)
	externalOrgID := "org_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	orgName := "acme-jwt-" + uuid.NewString()

	handler, cleanup, err := NewServer(Config{
		NexusAPIKeys: "nexus=secret",
		SaaS: SaaSConfig{
			DatabaseURL:        databaseURL,
			StripeSecretKey:    "sk_test_dummy",
			ClerkWebhookSecret: clerkSecret,
			JWTIssuer:          oidc.server.URL,
			JWTAudience:        "nexus-control-plane",
			JWTOrgClaim:        "o.id",
			JWTRoleClaim:       "o.rol",
			JWTScopesClaim:     "o.per",
			JWTActorClaim:      "sub",
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	defer cleanupAfterAsyncMetering(cleanup)

	webhookBody, err := json.Marshal(map[string]any{
		"type": "organization.created",
		"data": map[string]any{
			"id":   externalOrgID,
			"name": orgName,
		},
	})
	if err != nil {
		t.Fatalf("marshal webhook body: %v", err)
	}
	webhookReq := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", bytes.NewReader(webhookBody))
	applySvixHeaders(webhookReq, clerkSecret, webhookBody, time.Now())
	webhookRec := httptest.NewRecorder()
	handler.ServeHTTP(webhookRec, webhookReq)
	if webhookRec.Code != http.StatusOK {
		t.Fatalf("expected status %d from POST /webhooks/clerk, got %d body=%s", http.StatusOK, webhookRec.Code, webhookRec.Body.String())
	}

	token := oidc.issueToken(t, jwt.MapClaims{
		"iss": oidc.server.URL,
		"aud": "nexus-control-plane",
		"sub": "user_123",
		"exp": time.Now().Add(30 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
		"o": map[string]any{
			"id":  externalOrgID,
			"rol": "org:admin",
			"per": []string{"admin:console:read"},
		},
	})

	statusReq := httptest.NewRequest(http.MethodGet, "/billing/status", nil)
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected status %d from GET /billing/status, got %d body=%s", http.StatusOK, statusRec.Code, statusRec.Body.String())
	}

	var statusResp struct {
		PlanCode      string `json:"plan_code"`
		BillingStatus string `json:"billing_status"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode billing status response: %v", err)
	}
	if statusResp.PlanCode != "starter" || statusResp.BillingStatus != "trialing" {
		t.Fatalf("unexpected billing status payload: %#v", statusResp)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRec := httptest.NewRecorder()
	handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected status %d from GET /users/me, got %d body=%s", http.StatusOK, meRec.Code, meRec.Body.String())
	}

	var meResp struct {
		OrgID      string   `json:"org_id"`
		ExternalID string   `json:"external_id"`
		Role       string   `json:"role"`
		Scopes     []string `json:"scopes"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResp.Role != "admin" || meResp.ExternalID != "user_123" {
		t.Fatalf("unexpected me payload: %#v", meResp)
	}
	if len(meResp.Scopes) != 1 || meResp.Scopes[0] != "admin:console:read" {
		t.Fatalf("unexpected me scopes: %#v", meResp.Scopes)
	}
	if _, err := uuid.Parse(meResp.OrgID); err != nil {
		t.Fatalf("expected internal org UUID in me response, got %q", meResp.OrgID)
	}
}

func resolveSaaSTestDatabaseURL(t *testing.T) string {
	t.Helper()

	if value := strings.TrimSpace(os.Getenv("NEXUS_TEST_SAAS_DATABASE_URL")); value != "" {
		return value
	}
	return startPostgresTestContainer(t)
}

func provisionIsolatedSaaSSchema(t *testing.T, baseDatabaseURL string) string {
	t.Helper()

	adminDB, err := sharedpostgres.Open(context.Background(), baseDatabaseURL)
	if err != nil {
		t.Fatalf("open postgres for schema bootstrap: %v", err)
	}

	schemaName := "saas_it_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminDB.Exec(context.Background(), `CREATE SCHEMA "`+schemaName+`"`); err != nil {
		adminDB.Close()
		t.Fatalf("create test schema %q: %v", schemaName, err)
	}
	t.Cleanup(func() {
		if _, err := adminDB.Exec(context.Background(), `DROP SCHEMA IF EXISTS "`+schemaName+`" CASCADE`); err != nil {
			t.Fatalf("drop test schema %q: %v", schemaName, err)
		}
		adminDB.Close()
	})

	isolatedURL, err := withSearchPath(baseDatabaseURL, schemaName)
	if err != nil {
		t.Fatalf("build schema-scoped database url: %v", err)
	}
	return isolatedURL
}

func withSearchPath(rawURL, schema string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse database url: %w", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func startPostgresTestContainer(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available and NEXUS_TEST_SAAS_DATABASE_URL not set")
	}

	containerName := "nexus-saas-it-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	runCmd := exec.Command(
		"docker", "run", "--rm", "-d",
		"--name", containerName,
		"-e", "POSTGRES_DB=nexus_saas_test",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "127.0.0.1::5432",
		"postgres:16-alpine",
	)
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("start postgres test container: %v output=%s", err, string(out))
	}
	containerID := strings.TrimSpace(string(out))
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerID).Run()
	})

	portCmd := exec.Command("docker", "port", containerID, "5432/tcp")
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect postgres test container port: %v output=%s", err, string(portOut))
	}
	addr := strings.TrimSpace(string(portOut))
	parts := strings.Split(addr, ":")
	if len(parts) < 2 {
		t.Fatalf("unexpected docker port output: %q", addr)
	}
	port := strings.TrimSpace(parts[len(parts)-1])
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatalf("unexpected docker port %q: %v", port, err)
	}

	databaseURL := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%s/nexus_saas_test?sslmode=disable", port)
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sharedpostgres.Open(context.Background(), databaseURL)
		if err == nil {
			db.Close()
			return databaseURL
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("postgres test container did not become ready: %v", lastErr)
	return ""
}

type testOIDCIssuer struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	keyID      string
}

func newTestOIDCIssuer(t *testing.T) *testOIDCIssuer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	issuer := &testOIDCIssuer{
		privateKey: privateKey,
		keyID:      "test-kid",
	}
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	issuer.server = server
	t.Cleanup(server.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 server.URL,
			"authorization_endpoint": server.URL + "/authorize",
			"token_endpoint":         server.URL + "/token",
			"jwks_uri":               server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA",
				"kid": issuer.keyID,
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(issuer.privateKey.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(issuer.privateKey.PublicKey.E)).Bytes()),
			}},
		})
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return issuer
}

func (i *testOIDCIssuer) issueToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = i.keyID
	signed, err := token.SignedString(i.privateKey)
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

func newTestSvixSecret(t *testing.T) string {
	t.Helper()

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("generate svix secret: %v", err)
	}
	return "whsec_" + base64.StdEncoding.EncodeToString(raw)
}

func applySvixHeaders(req *http.Request, secret string, body []byte, now time.Time) {
	id := "evt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ts := strconv.FormatInt(now.Unix(), 10)
	req.Header.Set("svix-id", id)
	req.Header.Set("svix-timestamp", ts)
	req.Header.Set("svix-signature", "v1,"+signSvix(secret, id, ts, body))
}

func signSvix(secret, id, ts string, body []byte) string {
	secret = strings.TrimSpace(strings.TrimPrefix(secret, "whsec_"))
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		panic(err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id + "." + ts + "." + string(body)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func cleanupAfterAsyncMetering(cleanup func()) {
	time.Sleep(250 * time.Millisecond)
	cleanup()
}
