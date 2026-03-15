CREATE INDEX IF NOT EXISTS idx_audit_records_action_id ON audit_records (action_id) WHERE action_id IS NOT NULL
