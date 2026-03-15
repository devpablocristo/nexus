CREATE INDEX IF NOT EXISTS idx_audit_records_occurred_at ON audit_records (occurred_at DESC, created_at DESC)
