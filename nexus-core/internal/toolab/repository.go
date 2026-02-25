package toolab

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// RepositoryPort abstracts the data-state operations against the database.
type RepositoryPort interface {
	Fingerprint(ctx context.Context) (string, error)
	CreateSavepoint(ctx context.Context, id string) error
	RollbackToSavepoint(ctx context.Context, id string) error
	TruncateAll(ctx context.Context) error
}

// Repository implements RepositoryPort using GORM.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Fingerprint returns a deterministic SHA-256 hash of all public table names
// and their row counts. Same data produces the same fingerprint.
func (r *Repository) Fingerprint(ctx context.Context) (string, error) {
	sqlDB, err := r.db.DB()
	if err != nil {
		return "", fmt.Errorf("get sql.DB: %w", err)
	}

	rows, err := sqlDB.QueryContext(ctx,
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
		if err := sqlDB.QueryRowContext(ctx,
			fmt.Sprintf("SELECT count(*) FROM %q", tableName)).Scan(&count); err != nil {
			parts = append(parts, fmt.Sprintf("%s:err", tableName))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d", tableName, count))
	}

	sort.Strings(parts)
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("sha256:%x", hash), nil
}

// CreateSavepoint issues a PostgreSQL SAVEPOINT with the given ID.
func (r *Repository) CreateSavepoint(ctx context.Context, id string) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	_, err = sqlDB.ExecContext(ctx, fmt.Sprintf("SAVEPOINT %q", id))
	if err != nil {
		return fmt.Errorf("create savepoint: %w", err)
	}
	return nil
}

// RollbackToSavepoint rolls back to a previously created savepoint.
func (r *Repository) RollbackToSavepoint(ctx context.Context, id string) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	_, err = sqlDB.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %q", id))
	if err != nil {
		return fmt.Errorf("rollback to savepoint: %w", err)
	}
	return nil
}

// TruncateAll truncates every table in the public schema.
func (r *Repository) TruncateAll(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}

	rows, err := sqlDB.QueryContext(ctx,
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

	quoted := make([]string, len(tables))
	for i, t := range tables {
		quoted[i] = fmt.Sprintf("%q", t)
	}
	_, err = sqlDB.ExecContext(ctx,
		fmt.Sprintf("TRUNCATE %s CASCADE", strings.Join(quoted, ", ")))
	return err
}
