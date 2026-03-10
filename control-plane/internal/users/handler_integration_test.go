package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"nexus/pkg/types"
)

func TestHandler_UserAndAPIKeyEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newUsersTestDB(t)
	repo := NewRepository(db)
	uc := NewUsecases(repo)
	h := NewHandler(uc)

	orgID := uuid.New()
	if err := db.Exec(`INSERT INTO orgs(id, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, orgID.String(), "Nexus Org").Error; err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := uc.SyncUser(context.Background(), "user_abc", "alice@example.com", "Alice Doe", nil); err != nil {
		t.Fatalf("sync user: %v", err)
	}
	if _, err := uc.SyncMembership(context.Background(), orgID, "user_abc", "alice@example.com", "Alice Doe", nil, "admin"); err != nil {
		t.Fatalf("sync membership: %v", err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), orgID)
		c.Set(string(types.CtxKeyActor), "user_abc")
		c.Set(string(types.CtxKeyRole), "admin")
		c.Set(string(types.CtxKeyScopes), []string{"admin:console:read", "admin:console:write"})
		c.Next()
	})
	v1 := r.Group("/v1")
	h.Register(v1)

	t.Run("users me", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body["external_id"] != "user_abc" {
			t.Fatalf("unexpected external_id: %v", body["external_id"])
		}
	})

	t.Run("members list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/orgs/"+orgID.String()+"/members", nil)
		w := httptest.NewRecorder()
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
		if len(body.Items) != 1 {
			t.Fatalf("expected 1 member, got %d", len(body.Items))
		}
	})

	var createdKeyID string
	t.Run("api key lifecycle", func(t *testing.T) {
		payload := map[string]any{
			"name":   "tower-key",
			"scopes": []string{"audit:read", "admin:console:read"},
		}
		raw, _ := json.Marshal(payload)
		createReq := httptest.NewRequest(http.MethodPost, "/v1/orgs/"+orgID.String()+"/api-keys", bytes.NewReader(raw))
		createReq.Header.Set("Content-Type", "application/json")
		createW := httptest.NewRecorder()
		r.ServeHTTP(createW, createReq)
		if createW.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
		}
		var created map[string]any
		if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode created response: %v", err)
		}
		createdKeyID, _ = created["id"].(string)
		apiKeyRaw, _ := created["api_key"].(string)
		if createdKeyID == "" || apiKeyRaw == "" {
			t.Fatalf("expected id and api_key in create response")
		}

		listReq := httptest.NewRequest(http.MethodGet, "/v1/orgs/"+orgID.String()+"/api-keys", nil)
		listW := httptest.NewRecorder()
		r.ServeHTTP(listW, listReq)
		if listW.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", listW.Code, listW.Body.String())
		}

		rotateReq := httptest.NewRequest(http.MethodPost, "/v1/orgs/"+orgID.String()+"/api-keys/"+createdKeyID+"/rotate", nil)
		rotateW := httptest.NewRecorder()
		r.ServeHTTP(rotateW, rotateReq)
		if rotateW.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rotateW.Code, rotateW.Body.String())
		}
		var rotated map[string]any
		if err := json.Unmarshal(rotateW.Body.Bytes(), &rotated); err != nil {
			t.Fatalf("decode rotate response: %v", err)
		}
		if rotated["api_key"] == "" {
			t.Fatalf("expected rotated api_key")
		}

		deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/orgs/"+orgID.String()+"/api-keys/"+createdKeyID, nil)
		deleteW := httptest.NewRecorder()
		r.ServeHTTP(deleteW, deleteReq)
		if deleteW.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", deleteW.Code, deleteW.Body.String())
		}
	})
}
