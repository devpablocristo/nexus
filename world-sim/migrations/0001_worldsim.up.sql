CREATE TABLE IF NOT EXISTS world_runs (
  run_id text PRIMARY KEY,
  org_id text NOT NULL,
  seed bigint NOT NULL,
  config_hash text NOT NULL,
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_world_runs_org_created_at
  ON world_runs (org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS world_events (
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
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_world_events_run_seq
  ON world_events (run_id, seq);
CREATE INDEX IF NOT EXISTS idx_world_events_run_step
  ON world_events (run_id, step_id, seq);
CREATE INDEX IF NOT EXISTS idx_world_events_org_run_seq
  ON world_events (org_id, run_id, seq);

CREATE TABLE IF NOT EXISTS world_snapshots (
  run_id text NOT NULL REFERENCES world_runs(run_id) ON DELETE CASCADE,
  step_id bigint NOT NULL,
  state_json jsonb NOT NULL,
  state_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, step_id)
);

CREATE INDEX IF NOT EXISTS idx_world_snapshots_run_step
  ON world_snapshots (run_id, step_id DESC);
