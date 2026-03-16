CREATE INDEX IF NOT EXISTS idx_audit_records_incident_id ON audit_records (incident_id) WHERE incident_id IS NOT NULL;
