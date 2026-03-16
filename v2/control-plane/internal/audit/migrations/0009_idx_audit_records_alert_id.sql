CREATE INDEX IF NOT EXISTS idx_audit_records_alert_id ON audit_records (alert_id) WHERE alert_id IS NOT NULL;
