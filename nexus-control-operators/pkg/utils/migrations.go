package utils

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Migration struct {
	Version  int
	Name     string
	UpPath   string
	DownPath string
}

func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type pair struct{ up, down string }
	byVersion := map[int]*pair{}
	byName := map[int]string{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			continue
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if _, ok := byVersion[v]; !ok {
			byVersion[v] = &pair{}
			byName[v] = parts[1]
		}
		full := filepath.Join(dir, name)
		if strings.HasSuffix(name, ".up.sql") {
			byVersion[v].up = full
		} else if strings.HasSuffix(name, ".down.sql") {
			byVersion[v].down = full
		}
	}

	var out []Migration
	for v, p := range byVersion {
		if p.up == "" || p.down == "" {
			return nil, fmt.Errorf("migration %04d missing up or down file", v)
		}
		out = append(out, Migration{
			Version:  v,
			Name:     byName[v],
			UpPath:   p.up,
			DownPath: p.down,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

type Migrator struct {
	DB  *sql.DB
	Dir string
}

func (m Migrator) EnsureSchemaMigrations(ctx context.Context) error {
	_, err := m.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version int PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);`)
	return err
}

func (m Migrator) AppliedVersions(ctx context.Context) (map[int]struct{}, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]struct{}{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = struct{}{}
	}
	return out, rows.Err()
}

func (m Migrator) Up(ctx context.Context) error {
	if err := m.EnsureSchemaMigrations(ctx); err != nil {
		return err
	}
	migs, err := LoadMigrations(m.Dir)
	if err != nil {
		return err
	}
	applied, err := m.AppliedVersions(ctx)
	if err != nil {
		return err
	}

	for _, mig := range migs {
		if _, ok := applied[mig.Version]; ok {
			continue
		}
		sqlBytes, err := os.ReadFile(mig.UpPath)
		if err != nil {
			return err
		}
		if err := execSQL(ctx, m.DB, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply %04d up: %w", mig.Version, err)
		}
		_, err = m.DB.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES ($1, $2)`, mig.Version, time.Now().UTC())
		if err != nil {
			return err
		}
	}
	return nil
}

func (m Migrator) Down(ctx context.Context, steps int) error {
	if steps == 0 {
		steps = 1
	}
	if err := m.EnsureSchemaMigrations(ctx); err != nil {
		return err
	}
	migs, err := LoadMigrations(m.Dir)
	if err != nil {
		return err
	}

	byVersion := map[int]Migration{}
	for _, mig := range migs {
		byVersion[mig.Version] = mig
	}

	for i := 0; i < steps; i++ {
		var v int
		err := m.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
		if err != nil {
			return err
		}
		if v == 0 {
			return nil
		}
		mig, ok := byVersion[v]
		if !ok {
			return fmt.Errorf("no migration files for applied version %04d", v)
		}
		sqlBytes, err := os.ReadFile(mig.DownPath)
		if err != nil {
			return err
		}
		if err := execSQL(ctx, m.DB, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply %04d down: %w", mig.Version, err)
		}
		if _, err := m.DB.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mig.Version); err != nil {
			return err
		}
	}
	return nil
}

func execSQL(ctx context.Context, db *sql.DB, sqlText string) error {
	// Basic splitting is intentionally avoided; Postgres supports multi-statement exec.
	_, err := db.ExecContext(ctx, sqlText)
	return err
}
