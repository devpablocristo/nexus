ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS prev_event_hash text,
  ADD COLUMN IF NOT EXISTS event_hash text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_events_org_event_hash
  ON audit_events (org_id, event_hash)
  WHERE event_hash IS NOT NULL;

