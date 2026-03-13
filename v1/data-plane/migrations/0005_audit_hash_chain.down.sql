DROP INDEX IF EXISTS idx_audit_events_org_event_hash;

ALTER TABLE audit_events
  DROP COLUMN IF EXISTS event_hash,
  DROP COLUMN IF EXISTS prev_event_hash;

