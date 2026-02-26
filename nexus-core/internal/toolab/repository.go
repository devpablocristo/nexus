package toolab

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	domain "nexus-core/internal/toolab/usecases/domain"

	"gorm.io/gorm"
)

// RepositoryPort abstracts the data-state operations against the database.
type RepositoryPort interface {
	Fingerprint(ctx context.Context) (string, error)
	CreateSavepoint(ctx context.Context, id string) error
	RollbackToSavepoint(ctx context.Context, id string) error
	TruncateAll(ctx context.Context) error
	Schema(ctx context.Context) (*domain.SchemaResponse, error)
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

// Schema returns a best-effort public schema description compatible with TOOLAB Standard.
func (r *Repository) Schema(ctx context.Context) (*domain.SchemaResponse, error) {
	sqlDB, err := r.db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	// DB version is optional and best-effort.
	dbVersion := ""
	_ = sqlDB.QueryRowContext(ctx, "SHOW server_version").Scan(&dbVersion)

	type row struct {
		Table      string
		Column     string
		DataType   string
		IsNullable string
	}
	rows, err := sqlDB.QueryContext(ctx, `
		SELECT table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	defer rows.Close()

	pk := map[string]map[string]bool{}
	pkRows, err := sqlDB.QueryContext(ctx, `
		SELECT tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = 'public'
		  AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY tc.table_name, kcu.ordinal_position`)
	if err == nil {
		defer pkRows.Close()
		for pkRows.Next() {
			var tableName, columnName string
			if scanErr := pkRows.Scan(&tableName, &columnName); scanErr != nil {
				continue
			}
			if _, ok := pk[tableName]; !ok {
				pk[tableName] = map[string]bool{}
			}
			pk[tableName][columnName] = true
		}
	}

	fk := map[string]map[string]string{}
	fkRows, err := sqlDB.QueryContext(ctx, `
		SELECT tc.table_name, kcu.column_name, ccu.table_name, ccu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name
		 AND ccu.table_schema = tc.table_schema
		WHERE tc.table_schema = 'public'
		  AND tc.constraint_type = 'FOREIGN KEY'
		ORDER BY tc.table_name, kcu.ordinal_position`)
	if err == nil {
		defer fkRows.Close()
		for fkRows.Next() {
			var tableName, columnName, refTable, refColumn string
			if scanErr := fkRows.Scan(&tableName, &columnName, &refTable, &refColumn); scanErr != nil {
				continue
			}
			if _, ok := fk[tableName]; !ok {
				fk[tableName] = map[string]string{}
			}
			fk[tableName][columnName] = fmt.Sprintf("%s.%s", refTable, refColumn)
		}
	}

	rowHints := map[string]*int64{}
	hintsRows, err := sqlDB.QueryContext(ctx, `
		SELECT c.relname, c.reltuples::bigint
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		ORDER BY c.relname`)
	if err == nil {
		defer hintsRows.Close()
		for hintsRows.Next() {
			var tableName string
			var estimate int64
			if scanErr := hintsRows.Scan(&tableName, &estimate); scanErr != nil {
				continue
			}
			val := estimate
			rowHints[tableName] = &val
		}
	}

	entitiesByTable := map[string]*domain.EntityInfo{}
	tableOrder := []string{}
	for rows.Next() {
		var r row
		if scanErr := rows.Scan(&r.Table, &r.Column, &r.DataType, &r.IsNullable); scanErr != nil {
			return nil, scanErr
		}
		entity, ok := entitiesByTable[r.Table]
		if !ok {
			entity = &domain.EntityInfo{
				Name:    r.Table,
				Table:   r.Table,
				Columns: []domain.ColumnInfo{},
			}
			if hint, hasHint := rowHints[r.Table]; hasHint {
				entity.EstimatedRowCountHint = hint
			}
			entitiesByTable[r.Table] = entity
			tableOrder = append(tableOrder, r.Table)
		}
		col := domain.ColumnInfo{
			Name:     r.Column,
			Type:     r.DataType,
			Nullable: strings.EqualFold(r.IsNullable, "YES"),
		}
		if pk[r.Table][r.Column] {
			col.PK = true
		}
		if ref := fk[r.Table][r.Column]; ref != "" {
			col.FK = ref
		}
		entity.Columns = append(entity.Columns, col)
	}
	sort.Strings(tableOrder)
	entities := make([]domain.EntityInfo, 0, len(tableOrder))
	for _, tableName := range tableOrder {
		entities = append(entities, *entitiesByTable[tableName])
	}

	return &domain.SchemaResponse{
		Database: domain.DatabaseInfo{
			Type:       "postgres",
			Version:    dbVersion,
			SchemaName: "public",
		},
		Entities: entities,
	}, nil
}
