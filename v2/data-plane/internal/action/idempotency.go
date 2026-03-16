package action

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultIdempotencyTTL = 24 * time.Hour

// IdempotencyEntry stores a cached response for a given key.
type IdempotencyEntry struct {
	ActionID   string          `json:"action_id"`
	StatusCode int             `json:"status_code"`
	Response   json.RawMessage `json:"response"`
	ExpiresAt  time.Time       `json:"expires_at"`
}

// IdempotencyStore checks and stores idempotency keys.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (*IdempotencyEntry, error)
	Set(ctx context.Context, key string, entry IdempotencyEntry) error
	Purge(ctx context.Context) error
}

// InMemoryIdempotencyStore is used for testing and in-memory mode.
type InMemoryIdempotencyStore struct {
	mu    sync.RWMutex
	items map[string]IdempotencyEntry
}

func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{items: make(map[string]IdempotencyEntry)}
}

func (s *InMemoryIdempotencyStore) Get(_ context.Context, key string) (*IdempotencyEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.items[key]
	if !ok {
		return nil, nil
	}
	if time.Now().UTC().After(entry.ExpiresAt) {
		return nil, nil
	}
	return &entry, nil
}

func (s *InMemoryIdempotencyStore) Set(_ context.Context, key string, entry IdempotencyEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = entry
	return nil
}

func (s *InMemoryIdempotencyStore) Purge(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for k, v := range s.items {
		if now.After(v.ExpiresAt) {
			delete(s.items, k)
		}
	}
	return nil
}

// PostgresIdempotencyStore persists idempotency keys in PostgreSQL.
type PostgresIdempotencyStore struct {
	pool *pgxpool.Pool
}

func NewPostgresIdempotencyStore(pool *pgxpool.Pool) *PostgresIdempotencyStore {
	return &PostgresIdempotencyStore{pool: pool}
}

func (s *PostgresIdempotencyStore) Get(ctx context.Context, key string) (*IdempotencyEntry, error) {
	var entry IdempotencyEntry
	var actionID string
	err := s.pool.QueryRow(ctx,
		`SELECT action_id, status_code, response, expires_at
		 FROM idempotency_keys
		 WHERE key = $1 AND expires_at > NOW()`,
		key,
	).Scan(&actionID, &entry.StatusCode, &entry.Response, &entry.ExpiresAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	entry.ActionID = actionID
	return &entry, nil
}

func (s *PostgresIdempotencyStore) Set(ctx context.Context, key string, entry IdempotencyEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO idempotency_keys (key, action_id, status_code, response, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, NOW(), $5)
		 ON CONFLICT (key) DO NOTHING`,
		key, entry.ActionID, entry.StatusCode, entry.Response, entry.ExpiresAt,
	)
	return err
}

func (s *PostgresIdempotencyStore) Purge(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at < NOW()`)
	return err
}
