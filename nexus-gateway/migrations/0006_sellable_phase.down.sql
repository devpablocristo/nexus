ALTER TABLE audit_events
  DROP COLUMN IF EXISTS stage_durations_ms,
  DROP COLUMN IF EXISTS budget_remaining_ms_at_execute,
  DROP COLUMN IF EXISTS timeout_ms,
  DROP COLUMN IF EXISTS idempotency_outcome,
  DROP COLUMN IF EXISTS idempotency_present,
  DROP COLUMN IF EXISTS actor_scopes,
  DROP COLUMN IF EXISTS actor_role;

DROP INDEX IF EXISTS idx_idempotency_keys_org_tool_created_at;
DROP INDEX IF EXISTS idx_idempotency_keys_expires_at;
DROP TABLE IF EXISTS idempotency_keys;

ALTER TABLE tools
  DROP COLUMN IF EXISTS sensitivity;

