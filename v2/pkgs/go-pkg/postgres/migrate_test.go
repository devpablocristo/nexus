package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestLoadMigrationFilesSortsAndFiltersSQL(t *testing.T) {
	t.Parallel()

	migrations := fstest.MapFS{
		"migrations/0002_second.sql": {Data: []byte("CREATE INDEX two")},
		"migrations/0001_first.sql":  {Data: []byte("CREATE TABLE one")},
		"migrations/README.md":       {Data: []byte("ignore")},
		"migrations/nested/file.sql": {Data: []byte("ignore nested")},
	}

	items, err := loadMigrationFiles(migrations, "migrations")
	if err != nil {
		t.Fatalf("loadMigrationFiles returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected migrations count: %d", len(items))
	}
	if items[0].version != "0001_first.sql" || items[1].version != "0002_second.sql" {
		t.Fatalf("unexpected order: %#v", items)
	}
}

func TestLoadMigrationFilesRejectsEmptySQL(t *testing.T) {
	t.Parallel()

	migrations := fstest.MapFS{
		"migrations/0001_empty.sql": {Data: []byte("   ")},
	}

	_, err := loadMigrationFiles(migrations, "migrations")
	if err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("expected empty migration error, got %v", err)
	}
}

func TestMigrateUpRunsPendingMigrationsOnce(t *testing.T) {
	t.Parallel()

	migrations := fstest.MapFS{
		"migrations/0002_second.sql": {Data: []byte("CREATE INDEX second_idx")},
		"migrations/0001_first.sql":  {Data: []byte("CREATE TABLE first_table")},
	}
	db := newFakeMigrationDB()

	if err := MigrateUp(context.Background(), db, "control-plane/audit", migrations, "migrations"); err != nil {
		t.Fatalf("first MigrateUp returned error: %v", err)
	}
	if err := MigrateUp(context.Background(), db, "control-plane/audit", migrations, "migrations"); err != nil {
		t.Fatalf("second MigrateUp returned error: %v", err)
	}

	if len(db.ensureSchemaCalls) != 2 {
		t.Fatalf("unexpected ensure schema calls: %d", len(db.ensureSchemaCalls))
	}
	if got := strings.Join(db.appliedSQL, "|"); got != "CREATE TABLE first_table|CREATE INDEX second_idx" {
		t.Fatalf("unexpected applied sql order: %s", got)
	}
}

func TestMigrateUpRollsBackOnFailure(t *testing.T) {
	t.Parallel()

	migrations := fstest.MapFS{
		"migrations/0001_first.sql": {Data: []byte("CREATE TABLE first_table")},
		"migrations/0002_fail.sql":  {Data: []byte("CREATE INDEX broken_idx")},
	}
	db := newFakeMigrationDB()
	db.failSQL = "CREATE INDEX broken_idx"

	err := MigrateUp(context.Background(), db, "control-plane/audit", migrations, "migrations")
	if err == nil || !strings.Contains(err.Error(), "execute migration") {
		t.Fatalf("expected execute migration error, got %v", err)
	}
	if _, ok := db.appliedVersions["control-plane/audit|0002_fail.sql"]; ok {
		t.Fatalf("failed migration was marked as applied")
	}
}

type fakeMigrationDB struct {
	ensureSchemaCalls []string
	appliedSQL        []string
	appliedVersions   map[string]struct{}
	failSQL           string
}

func newFakeMigrationDB() *fakeMigrationDB {
	return &fakeMigrationDB{
		appliedVersions: make(map[string]struct{}),
	}
}

func (db *fakeMigrationDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	db.ensureSchemaCalls = append(db.ensureSchemaCalls, strings.TrimSpace(sql))
	return pgconn.NewCommandTag("CREATE TABLE"), nil
}

func (db *fakeMigrationDB) Begin(_ context.Context) (migrationTx, error) {
	return &fakeMigrationTx{db: db}, nil
}

type fakeMigrationTx struct {
	db             *fakeMigrationDB
	pendingVersion string
}

func (tx *fakeMigrationTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	sql = strings.TrimSpace(sql)
	if strings.HasPrefix(sql, "INSERT INTO schema_migrations") {
		scope := args[0].(string)
		version := args[1].(string)
		key := fmt.Sprintf("%s|%s", scope, version)
		if _, ok := tx.db.appliedVersions[key]; ok {
			return pgconn.NewCommandTag("INSERT 0 0"), nil
		}
		tx.pendingVersion = key
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}
	if tx.db.failSQL != "" && sql == tx.db.failSQL {
		return pgconn.CommandTag{}, errors.New("boom")
	}
	tx.db.appliedSQL = append(tx.db.appliedSQL, sql)
	return pgconn.NewCommandTag("CREATE TABLE"), nil
}

func (tx *fakeMigrationTx) Commit(_ context.Context) error {
	if tx.pendingVersion != "" {
		tx.db.appliedVersions[tx.pendingVersion] = struct{}{}
	}
	return nil
}

func (tx *fakeMigrationTx) Rollback(_ context.Context) error {
	return nil
}

var _ fs.FS
