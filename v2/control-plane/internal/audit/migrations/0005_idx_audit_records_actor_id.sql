CREATE INDEX IF NOT EXISTS idx_audit_records_actor_id ON audit_records (actor_id) WHERE actor_id IS NOT NULL
