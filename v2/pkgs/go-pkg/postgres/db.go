package postgres

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type migrationTx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type migrationDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (migrationTx, error)
}

type DB struct {
	pool *pgxpool.Pool
}

type Config struct {
	ApplicationName   string
	MinConns          int32
	MaxConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
	StatementTimeout  time.Duration
}

func DefaultConfig(applicationName string) Config {
	return Config{
		ApplicationName:   strings.TrimSpace(applicationName),
		MinConns:          1,
		MaxConns:          8,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    5 * time.Second,
		StatementTimeout:  5 * time.Second,
	}
}

func ConfigFromEnv(prefix, applicationName string) (Config, error) {
	config := DefaultConfig(applicationName)
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return config, nil
	}

	var err error
	config.MinConns, err = int32FromEnv(prefix+"_MIN_CONNS", config.MinConns)
	if err != nil {
		return Config{}, err
	}
	config.MaxConns, err = int32FromEnv(prefix+"_MAX_CONNS", config.MaxConns)
	if err != nil {
		return Config{}, err
	}
	config.MaxConnLifetime, err = durationFromEnv(prefix+"_MAX_CONN_LIFETIME", config.MaxConnLifetime)
	if err != nil {
		return Config{}, err
	}
	config.MaxConnIdleTime, err = durationFromEnv(prefix+"_MAX_CONN_IDLE_TIME", config.MaxConnIdleTime)
	if err != nil {
		return Config{}, err
	}
	config.HealthCheckPeriod, err = durationFromEnv(prefix+"_HEALTH_CHECK_PERIOD", config.HealthCheckPeriod)
	if err != nil {
		return Config{}, err
	}
	config.ConnectTimeout, err = durationFromEnv(prefix+"_CONNECT_TIMEOUT", config.ConnectTimeout)
	if err != nil {
		return Config{}, err
	}
	config.StatementTimeout, err = durationFromEnv(prefix+"_STATEMENT_TIMEOUT", config.StatementTimeout)
	if err != nil {
		return Config{}, err
	}

	if config.MinConns < 0 {
		return Config{}, fmt.Errorf("%s_MIN_CONNS must be >= 0", prefix)
	}
	if config.MaxConns <= 0 {
		return Config{}, fmt.Errorf("%s_MAX_CONNS must be > 0", prefix)
	}
	if config.MinConns > config.MaxConns {
		return Config{}, fmt.Errorf("%s_MIN_CONNS must be <= %s_MAX_CONNS", prefix, prefix)
	}
	return config, nil
}

func Open(ctx context.Context, databaseURL string) (*DB, error) {
	return OpenWithConfig(ctx, databaseURL, DefaultConfig(""))
}

func OpenWithConfig(ctx context.Context, databaseURL string, config Config) (*DB, error) {
	poolConfig, err := buildPoolConfig(databaseURL, config)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres pool: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (db *DB) Pool() *pgxpool.Pool {
	if db == nil {
		return nil
	}
	return db.pool
}

func (db *DB) Close() {
	if db == nil || db.pool == nil {
		return
	}
	db.pool.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("postgres pool is nil")
	}
	return db.pool.Ping(ctx)
}

func (db *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return db.pool.Exec(ctx, sql, args...)
}

func (db *DB) Begin(ctx context.Context) (migrationTx, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

var _ migrationDB = (*DB)(nil)
var _ migrationTx = (pgx.Tx)(nil)

func buildPoolConfig(databaseURL string, config Config) (*pgxpool.Config, error) {
	poolConfig, err := pgxpool.ParseConfig(strings.TrimSpace(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MaxConnLifetime = config.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = config.HealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = config.ConnectTimeout
	if name := strings.TrimSpace(config.ApplicationName); name != "" {
		poolConfig.ConnConfig.RuntimeParams["application_name"] = name
	}
	if config.StatementTimeout > 0 {
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(config.StatementTimeout.Milliseconds(), 10)
	}
	return poolConfig, nil
}

func int32FromEnv(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return int32(parsed), nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
