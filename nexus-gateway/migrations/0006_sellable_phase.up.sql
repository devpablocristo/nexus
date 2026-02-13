ALTER TABLE tools
  ADD COLUMN IF NOT EXISTS sensitivity text NOT NULL DEFAULT 'low' CHECK (sensitivity IN ('low','medium','high'));

CREATE TABLE IF NOT EXISTS idempotency_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  tool_name text NOT NULL,
  idempotency_key text NOT NULL,
  request_fingerprint text NOT NULL,
  status text NOT NULL CHECK (status IN ('IN_PROGRESS','COMPLETED','FAILED')),
  response_redacted_json jsonb,
  error_code text,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  UNIQUE (org_id, tool_name, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_org_tool_created_at ON idempotency_keys (org_id, tool_name, created_at DESC);

ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS actor_role text,
  ADD COLUMN IF NOT EXISTS actor_scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS idempotency_present bool NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS idempotency_outcome text NOT NULL DEFAULT 'SKIPPED_NOT_WRITE' CHECK (
    idempotency_outcome IN ('NEW','REPLAY','IN_PROGRESS','CONFLICT','MISSING_REQUIRED','SKIPPED_NOT_WRITE')
  ),
  ADD COLUMN IF NOT EXISTS timeout_ms int,
  ADD COLUMN IF NOT EXISTS budget_remaining_ms_at_execute int,
  ADD COLUMN IF NOT EXISTS stage_durations_ms jsonb NOT NULL DEFAULT '{}'::jsonb;

