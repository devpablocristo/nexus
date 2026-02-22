package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"world-sim/internal/model"
	"world-sim/internal/sim"
)

type Store struct {
	db *sql.DB
}

func Open(databaseURL string) (*Store, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS world_runs (
			run_id text PRIMARY KEY,
			org_id text NOT NULL,
			seed bigint NOT NULL,
			config_hash text NOT NULL,
			config_json jsonb NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_world_runs_org_created_at ON world_runs (org_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS world_events (
			id bigserial PRIMARY KEY,
			run_id text NOT NULL REFERENCES world_runs(run_id) ON DELETE CASCADE,
			step_id bigint NOT NULL,
			seq bigint NOT NULL,
			org_id text NOT NULL,
			agent_id text NOT NULL,
			tool_name text NOT NULL,
			payload_json jsonb NOT NULL,
			request_id text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_world_events_run_seq ON world_events (run_id, seq)`,
		`CREATE INDEX IF NOT EXISTS idx_world_events_run_step ON world_events (run_id, step_id, seq)`,
		`CREATE INDEX IF NOT EXISTS idx_world_events_org_run_seq ON world_events (org_id, run_id, seq)`,
		`CREATE TABLE IF NOT EXISTS world_snapshots (
			run_id text NOT NULL REFERENCES world_runs(run_id) ON DELETE CASCADE,
			step_id bigint NOT NULL,
			state_json jsonb NOT NULL,
			state_hash text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (run_id, step_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_world_snapshots_run_step ON world_snapshots (run_id, step_id DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateRun(ctx context.Context, run model.Run) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO world_runs(run_id, org_id, seed, config_hash, config_json, created_at)
		VALUES($1,$2,$3,$4,$5,$6)
	`, run.RunID, run.OrgID, run.Seed, run.ConfigHash, run.ConfigJSON, run.CreatedAt.UTC())
	return err
}

func (s *Store) GetRun(ctx context.Context, orgID, runID string) (model.Run, error) {
	var run model.Run
	err := s.db.QueryRowContext(ctx, `
		SELECT run_id, org_id, seed, config_hash, config_json, created_at
		FROM world_runs WHERE org_id = $1 AND run_id = $2
	`, orgID, runID).Scan(&run.RunID, &run.OrgID, &run.Seed, &run.ConfigHash, &run.ConfigJSON, &run.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Run{}, fmt.Errorf("run not found")
		}
		return model.Run{}, err
	}
	return run, nil
}

func (s *Store) ListRuns(ctx context.Context, orgID string, limit int, cursor string) ([]model.Run, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	q := `
		SELECT run_id, org_id, seed, config_hash, config_json, created_at
		FROM world_runs
		WHERE org_id = $1
	`
	args := []any{orgID}
	if cursor != "" {
		q += ` AND created_at < COALESCE((SELECT created_at FROM world_runs WHERE run_id = $2), now())`
		args = append(args, cursor)
	}
	q += ` ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Run{}
	for rows.Next() {
		var run model.Run
		if err := rows.Scan(&run.RunID, &run.OrgID, &run.Seed, &run.ConfigHash, &run.ConfigJSON, &run.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (s *Store) NextSeq(ctx context.Context, runID string) (int64, error) {
	var seq int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM world_events WHERE run_id = $1`, runID).Scan(&seq)
	if err != nil {
		return 0, err
	}
	return seq + 1, nil
}

func (s *Store) InsertEvent(ctx context.Context, ev model.Event) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO world_events(run_id, step_id, seq, org_id, agent_id, tool_name, payload_json, request_id, created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, ev.RunID, ev.StepID, ev.Seq, ev.OrgID, ev.AgentID, ev.ToolName, ev.Payload, ev.RequestID, time.Now().UTC())
	return err
}

func (s *Store) InsertEvents(ctx context.Context, events []model.Event) error {
	for _, ev := range events {
		if err := s.InsertEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SaveSnapshot(ctx context.Context, snap model.Snapshot) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO world_snapshots(run_id, step_id, state_json, state_hash, created_at)
		VALUES($1,$2,$3,$4,$5)
		ON CONFLICT (run_id, step_id)
		DO UPDATE SET state_json=EXCLUDED.state_json, state_hash=EXCLUDED.state_hash, created_at=EXCLUDED.created_at
	`, snap.RunID, snap.StepID, snap.StateJSON, snap.StateHash, time.Now().UTC())
	return err
}

func (s *Store) LatestSnapshot(ctx context.Context, runID string, stepID *int64) (model.Snapshot, error) {
	q := `
		SELECT run_id, step_id, state_json, state_hash, created_at
		FROM world_snapshots WHERE run_id = $1
	`
	args := []any{runID}
	if stepID != nil {
		q += ` AND step_id <= $2`
		args = append(args, *stepID)
	}
	q += ` ORDER BY step_id DESC LIMIT 1`
	var snap model.Snapshot
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&snap.RunID, &snap.StepID, &snap.StateJSON, &snap.StateHash, &snap.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Snapshot{}, fmt.Errorf("snapshot not found")
		}
		return model.Snapshot{}, err
	}
	return snap, nil
}

func (s *Store) ListEvents(ctx context.Context, orgID, runID string, fromSeq int64, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, step_id, seq, org_id, agent_id, tool_name, payload_json, request_id, created_at
		FROM world_events
		WHERE org_id = $1 AND run_id = $2 AND seq > $3
		ORDER BY seq ASC LIMIT $4
	`, orgID, runID, fromSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Event{}
	for rows.Next() {
		var ev model.Event
		if err := rows.Scan(&ev.ID, &ev.RunID, &ev.StepID, &ev.Seq, &ev.OrgID, &ev.AgentID, &ev.ToolName, &ev.Payload, &ev.RequestID, &ev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) ListMoveCalls(ctx context.Context, orgID, runID string) ([]model.Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, step_id, seq, org_id, agent_id, tool_name, payload_json, request_id, created_at
		FROM world_events
		WHERE org_id = $1 AND run_id = $2 AND tool_name = 'world.move'
		ORDER BY step_id ASC, seq ASC
	`, orgID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Event{}
	for rows.Next() {
		var ev model.Event
		if err := rows.Scan(&ev.ID, &ev.RunID, &ev.StepID, &ev.Seq, &ev.OrgID, &ev.AgentID, &ev.ToolName, &ev.Payload, &ev.RequestID, &ev.CreatedAt); err != nil {
			return nil, err
		}
		var payload map[string]any
		_ = json.Unmarshal(ev.Payload, &payload)
		if et, _ := payload["event_type"].(string); et == "tool.called" {
			out = append(out, ev)
		}
	}
	return out, rows.Err()
}

func (s *Store) DeleteSnapshots(ctx context.Context, runID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM world_snapshots WHERE run_id = $1`, runID)
	return err
}

func BuildRun(seed int64, orgID, runID, configHash string, configJSON []byte) model.Run {
	return model.Run{
		RunID:      runID,
		OrgID:      orgID,
		Seed:       seed,
		ConfigHash: configHash,
		ConfigJSON: configJSON,
		CreatedAt:  time.Now().UTC(),
	}
}

func BuildSnapshot(runID string, stepID int64, rawState []byte, hash string) model.Snapshot {
	return model.Snapshot{RunID: runID, StepID: stepID, StateJSON: rawState, StateHash: hash, CreatedAt: time.Now().UTC()}
}

func DecodeRunConfig(run model.Run) (sim.WorldConfig, error) {
	var cfg sim.WorldConfig
	if err := json.Unmarshal(run.ConfigJSON, &cfg); err != nil {
		return sim.WorldConfig{}, err
	}
	return cfg, nil
}
