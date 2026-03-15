package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

type migrationFile struct {
	version string
	sql     string
}

func MigrateUp(ctx context.Context, db migrationDB, scope string, migrations fs.FS, dir string) error {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return fmt.Errorf("migration scope required")
	}
	items, err := loadMigrationFiles(migrations, dir)
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			scope text NOT NULL,
			version text NOT NULL,
			applied_at timestamptz NOT NULL,
			PRIMARY KEY (scope, version)
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	for _, item := range items {
		if err := applyMigration(ctx, db, scope, item); err != nil {
			return fmt.Errorf("apply migration %s/%s: %w", scope, item.version, err)
		}
	}
	return nil
}

func loadMigrationFiles(migrations fs.FS, dir string) ([]migrationFile, error) {
	dir = strings.Trim(strings.TrimSpace(dir), "/")
	entries, err := fs.ReadDir(migrations, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}
	items := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		fullPath := entry.Name()
		if dir != "" {
			fullPath = path.Join(dir, entry.Name())
		}
		body, err := fs.ReadFile(migrations, fullPath)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", fullPath, err)
		}
		sql := strings.TrimSpace(string(body))
		if sql == "" {
			return nil, fmt.Errorf("migration %q is empty", fullPath)
		}
		items = append(items, migrationFile{
			version: entry.Name(),
			sql:     sql,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].version < items[j].version
	})
	return items, nil
}

func applyMigration(ctx context.Context, db migrationDB, scope string, item migrationFile) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	tag, err := tx.Exec(ctx, `
		INSERT INTO schema_migrations (scope, version, applied_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (scope, version) DO NOTHING
	`, scope, item.version)
	if err != nil {
		return fmt.Errorf("register migration: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, item.sql); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	committed = true
	return nil
}

var _ pgconn.CommandTag
