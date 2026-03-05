package users

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepository_UserAndMemberLifecycle(t *testing.T) {
	db := newUsersTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	orgID, err := repo.UpsertOrgByName(ctx, "Acme")
	if err != nil {
		t.Fatalf("upsert org: %v", err)
	}

	user, err := repo.UpsertUser(ctx, "user_123", "alice@example.com", "Alice", nil)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if user.ExternalID != "user_123" {
		t.Fatalf("unexpected external_id: %s", user.ExternalID)
	}

	member, err := repo.UpsertOrgMember(ctx, orgID, user.ID, "admin")
	if err != nil {
		t.Fatalf("upsert member: %v", err)
	}
	if member.Role != "admin" {
		t.Fatalf("unexpected role: %s", member.Role)
	}

	members, err := repo.ListOrgMembers(ctx, orgID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].User.Email != "alice@example.com" {
		t.Fatalf("unexpected user email: %s", members[0].User.Email)
	}
}

func TestRepository_APIKeyLifecycle(t *testing.T) {
	db := newUsersTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	orgID := uuid.New()
	if err := db.Exec(`INSERT INTO orgs(id, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, orgID.String(), "Org Keys").Error; err != nil {
		t.Fatalf("seed org: %v", err)
	}

	created, err := repo.CreateAPIKey(ctx, orgID, CreateAPIKeyInput{
		Name:   "automation",
		Scopes: []string{"audit:read", "admin:console:read"},
	})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if created.Raw == "" {
		t.Fatalf("expected raw api key")
	}

	keys, err := repo.ListAPIKeys(ctx, orgID)
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 api key, got %d", len(keys))
	}
	if keys[0].Name != "automation" {
		t.Fatalf("unexpected name: %s", keys[0].Name)
	}

	rotated, err := repo.RotateAPIKey(ctx, orgID, created.Key.ID)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	if rotated == "" || rotated == created.Raw {
		t.Fatalf("expected new rotated raw key")
	}

	if err := repo.DeleteAPIKey(ctx, orgID, created.Key.ID); err != nil {
		t.Fatalf("delete api key: %v", err)
	}
	keys, err = repo.ListAPIKeys(ctx, orgID)
	if err != nil {
		t.Fatalf("list api keys after delete: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 api keys, got %d", len(keys))
	}
}

func newUsersTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
