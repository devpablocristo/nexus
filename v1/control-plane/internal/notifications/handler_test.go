package notifications

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	notificationdomain "control-plane/internal/notifications/usecases/domain"
	"nexus/pkg/types"
)

func TestHandler_PreferencesLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newNotificationsHTTPTestDB(t)
	orgID := uuid.New()
	userExternalID := "user_http"
	seedNotificationsHTTPData(t, db, orgID, userExternalID, "http-user@nexus.test")

	repo := NewRepository(db)
	uc := &Usecases{
		repo:             repo,
		sender:           NewNoopSender(zerolog.Nop()),
		logger:           zerolog.Nop(),
		towerBaseURL:     "http://localhost:5173",
		preferencesURL:   "http://localhost:5173/settings/notifications",
		defaultActionURL: "http://localhost:5173/tools",
		now:              func() time.Time { return time.Date(2026, 3, 5, 15, 0, 0, 0, time.UTC) },
	}
	h := NewHandler(uc)

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyActor), userExternalID)
		c.Set(string(types.CtxKeyOrgID), orgID)
		c.Next()
	})
	h.Register(v1)

	t.Run("get defaults", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/notifications/preferences", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		expected := len(notificationdomain.OrderedNotificationTypes())
		if len(body.Items) != expected {
			t.Fatalf("expected %d preferences, got %d", expected, len(body.Items))
		}
	})

	t.Run("put updates", func(t *testing.T) {
		payload := map[string]any{
			"items": []map[string]any{
				{"notification_type": "payment_failed", "enabled": false},
				{"notification_type": "incident_closed", "enabled": false},
			},
		}
		raw, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/v1/notifications/preferences", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}

		wGet := httptest.NewRecorder()
		reqGet := httptest.NewRequest(http.MethodGet, "/v1/notifications/preferences", nil)
		r.ServeHTTP(wGet, reqGet)
		if wGet.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", wGet.Code, wGet.Body.String())
		}
		var body struct {
			Items []struct {
				NotificationType string `json:"notification_type"`
				Enabled          bool   `json:"enabled"`
			} `json:"items"`
		}
		if err := json.Unmarshal(wGet.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		seenDisabled := map[string]bool{}
		for _, item := range body.Items {
			if !item.Enabled {
				seenDisabled[item.NotificationType] = true
			}
		}
		if !seenDisabled["payment_failed"] || !seenDisabled["incident_closed"] {
			t.Fatalf("expected updated disabled preferences, got %+v", seenDisabled)
		}
	})

	t.Run("in-app list unread and mark read", func(t *testing.T) {
		notifID := uuid.New()
		if err := db.Exec(
			`INSERT INTO in_app_notifications(id,org_id,actor_id,type,title,body,read_at,created_at) VALUES (?,?,?,?,?,?,NULL,CURRENT_TIMESTAMP)`,
			notifID.String(),
			orgID.String(),
			userExternalID,
			"payment_failed",
			"Payment failed",
			"Please update payment method",
		).Error; err != nil {
			t.Fatalf("insert in_app_notifications: %v", err)
		}

		wCount := httptest.NewRecorder()
		reqCount := httptest.NewRequest(http.MethodGet, "/v1/notifications/unread-count", nil)
		r.ServeHTTP(wCount, reqCount)
		if wCount.Code != http.StatusOK {
			t.Fatalf("expected unread-count 200, got %d: %s", wCount.Code, wCount.Body.String())
		}
		var unread struct {
			Count int64 `json:"count"`
		}
		if err := json.Unmarshal(wCount.Body.Bytes(), &unread); err != nil {
			t.Fatalf("decode unread-count: %v", err)
		}
		if unread.Count != 1 {
			t.Fatalf("expected unread count 1, got %d", unread.Count)
		}

		wList := httptest.NewRecorder()
		reqList := httptest.NewRequest(http.MethodGet, "/v1/notifications?limit=10", nil)
		r.ServeHTTP(wList, reqList)
		if wList.Code != http.StatusOK {
			t.Fatalf("expected list 200, got %d: %s", wList.Code, wList.Body.String())
		}
		var listed struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(wList.Body.Bytes(), &listed); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		if len(listed.Items) != 1 {
			t.Fatalf("expected 1 in-app item, got %d", len(listed.Items))
		}

		wRead := httptest.NewRecorder()
		reqRead := httptest.NewRequest(http.MethodPut, "/v1/notifications/"+notifID.String()+"/read", bytes.NewReader([]byte("{}")))
		reqRead.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(wRead, reqRead)
		if wRead.Code != http.StatusNoContent {
			t.Fatalf("expected mark-read 204, got %d: %s", wRead.Code, wRead.Body.String())
		}

		wCount2 := httptest.NewRecorder()
		reqCount2 := httptest.NewRequest(http.MethodGet, "/v1/notifications/unread-count", nil)
		r.ServeHTTP(wCount2, reqCount2)
		if wCount2.Code != http.StatusOK {
			t.Fatalf("expected unread-count 200, got %d: %s", wCount2.Code, wCount2.Body.String())
		}
		var unread2 struct {
			Count int64 `json:"count"`
		}
		if err := json.Unmarshal(wCount2.Body.Bytes(), &unread2); err != nil {
			t.Fatalf("decode unread-count after read: %v", err)
		}
		if unread2.Count != 0 {
			t.Fatalf("expected unread count 0, got %d", unread2.Count)
		}
	})
}

func newNotificationsHTTPTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	stmts := []string{
		`CREATE TABLE orgs (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, external_id TEXT NOT NULL UNIQUE, email TEXT NOT NULL UNIQUE, name TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE org_members (id TEXT PRIMARY KEY, org_id TEXT NOT NULL, user_id TEXT NOT NULL, role TEXT NOT NULL, joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(org_id, user_id))`,
		`CREATE TABLE notification_preferences (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, notification_type TEXT NOT NULL, channel TEXT NOT NULL DEFAULT 'email', enabled BOOLEAN NOT NULL DEFAULT 1, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, notification_type, channel))`,
		`CREATE TABLE notification_log (id TEXT PRIMARY KEY, org_id TEXT NOT NULL, user_id TEXT NULL, notification_type TEXT NOT NULL, channel TEXT NOT NULL DEFAULT 'email', recipient TEXT NOT NULL, subject TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'sent', dedup_key TEXT NULL UNIQUE, error_message TEXT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE in_app_notifications (id TEXT PRIMARY KEY, org_id TEXT NOT NULL, actor_id TEXT NOT NULL DEFAULT '', type TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '', read_at DATETIME NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func seedNotificationsHTTPData(t *testing.T, db *gorm.DB, orgID uuid.UUID, externalID, email string) {
	t.Helper()
	userID := uuid.New()
	memberID := uuid.New()
	if err := db.Exec(`INSERT INTO orgs(id,name,created_at) VALUES (?,?,CURRENT_TIMESTAMP)`, orgID.String(), "HTTP Org").Error; err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if err := db.Exec(`INSERT INTO users(id,external_id,email,name,created_at,updated_at) VALUES (?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, userID.String(), externalID, email, "HTTP User").Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Exec(`INSERT INTO org_members(id,org_id,user_id,role,joined_at) VALUES (?,?,?,?,CURRENT_TIMESTAMP)`, memberID.String(), orgID.String(), userID.String(), "admin").Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
}
