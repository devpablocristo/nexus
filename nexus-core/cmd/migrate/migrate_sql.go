package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"nexus-core/pkg/utils"
)

type Runner struct {
	DB  *sql.DB
	Dir string
}

func (r Runner) Up(ctx context.Context) error {
	m := utils.Migrator{DB: r.DB, Dir: r.Dir}
	return m.Up(ctx)
}

func (r Runner) Down(ctx context.Context, steps int) error {
	m := utils.Migrator{DB: r.DB, Dir: r.Dir}
	return m.Down(ctx, steps)
}

func OpenDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(5)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return db, nil
}
