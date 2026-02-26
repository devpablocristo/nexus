package toolab

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// defaultDBState implements StateProvider using a *sql.DB.
// It provides a basic implementation suitable for PostgreSQL.
type defaultDBState struct {
	db *sql.DB
}

func (s *defaultDBState) Fingerprint(ctx context.Context) (string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		 ORDER BY table_name`)
	if err != nil {
		return "", fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return "", err
		}
		var count int64
		err := s.db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT count(*) FROM %q", tableName)).Scan(&count)
		if err != nil {
			parts = append(parts, fmt.Sprintf("%s:err", tableName))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d", tableName, count))
	}
	sort.Strings(parts)
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("sha256:%x", hash), nil
}

func (s *defaultDBState) Snapshot(ctx context.Context, label string) (string, string, error) {
	fp, err := s.Fingerprint(ctx)
	if err != nil {
		return "", "", err
	}
	id := fmt.Sprintf("snap_%s", time.Now().UTC().Format("20060102_150405"))
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("SAVEPOINT %q", id))
	if err != nil {
		return "", "", fmt.Errorf("create savepoint: %w", err)
	}
	return id, fp, nil
}

func (s *defaultDBState) Restore(ctx context.Context, snapshotID string) error {
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %q", snapshotID))
	if err != nil {
		return fmt.Errorf("rollback to savepoint: %w", err)
	}
	return nil
}

func (s *defaultDBState) Reset(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		 ORDER BY table_name`)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		tables = append(tables, t)
	}
	if len(tables) == 0 {
		return nil
	}
	_, err = s.db.ExecContext(ctx,
		fmt.Sprintf("TRUNCATE %s CASCADE", strings.Join(quoteAll(tables), ", ")))
	return err
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}
