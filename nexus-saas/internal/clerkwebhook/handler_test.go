package clerkwebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"nexus-saas/cmd/config"
	"nexus-saas/internal/users"
)

func TestVerifySvix_ValidAndInvalid(t *testing.T) {
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("super-secret"))
	body := []byte(`{"type":"user.created"}`)
	id := "msg_123"
	ts := "1730000000"
	sig := signSvix(t, secret, id, ts, body)

	now := func() time.Time { return time.Unix(1730000000, 0) }
	if err := verifySvix(secret, id, ts, "v1,"+sig, body, now); err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}
	if err := verifySvix(secret, id, ts, "v1,invalid", body, now); err == nil {
		t.Fatalf("expected invalid signature error")
	}
}

func TestHandler_ProcessesClerkWebhookEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newWebhookTestDB(t)
	uc := users.NewUsecases(users.NewRepository(db))

	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("super-secret"))
	cfg := config.ServiceConfig{ClerkWebhookSecret: secret}
	h := NewHandler(cfg, uc, zerolog.Nop())
	h.now = func() time.Time { return time.Unix(1730000000, 0) }

	r := gin.New()
	v1 := r.Group("/v1")
	h.Register(v1)

	userPayload := map[string]any{
		"type": "user.created",
		"data": map[string]any{
			"id":                       "user_1",
			"first_name":               "Alice",
			"last_name":                "Operator",
			"primary_email_address_id": "e_1",
			"email_addresses": []map[string]any{
				{"id": "e_1", "email_address": "alice@example.com"},
			},
			"image_url": "https://cdn.example.com/u1.png",
		},
	}
	postWebhook(t, r, secret, userPayload)
	me, ok, err := users.NewRepository(db).FindUserByExternalID(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if !ok || me.Email != "alice@example.com" {
		t.Fatalf("expected synced user with email alice@example.com")
	}

	membershipPayload := map[string]any{
		"type": "organizationMembership.created",
		"data": map[string]any{
			"id":   "om_1",
			"role": "org:admin",
			"organization": map[string]any{
				"id":   "org_1",
				"name": "Acme Security",
				"slug": "acme-security",
			},
			"public_user_data": map[string]any{
				"user_id":    "user_1",
				"identifier": "alice@example.com",
			},
		},
	}
	postWebhook(t, r, secret, membershipPayload)

	repo := users.NewRepository(db)
	orgID, err := repo.UpsertOrgByName(context.Background(), "Acme Security")
	if err != nil {
		t.Fatalf("upsert org lookup: %v", err)
	}
	members, err := repo.ListOrgMembers(context.Background(), orgID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Role != "admin" {
		t.Fatalf("expected admin role, got %s", members[0].Role)
	}
}

func postWebhook(t *testing.T, r *gin.Engine, secret string, payload map[string]any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	id := "msg_123"
	ts := "1730000000"
	sig := signSvix(t, secret, id, ts, raw)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clerk", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerSvixID, id)
	req.Header.Set(headerSvixTimestamp, ts)
	req.Header.Set(headerSvixSignature, "v1,"+sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func signSvix(t *testing.T, secret, id, ts string, payload []byte) string {
	t.Helper()
	key, err := decodeSvixSecret(secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id + "." + ts + "." + string(payload)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func newWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	stmts := []string{
		`CREATE TABLE orgs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			external_id TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			avatar_url TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE org_members (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'secops',
			joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, user_id)
		)`,
		`CREATE TABLE org_api_keys (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			api_key_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE org_api_key_scopes (
			id TEXT PRIMARY KEY,
			api_key_id TEXT NOT NULL,
			scope TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}
