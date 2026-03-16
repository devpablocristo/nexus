ALTER TABLE audit_records ADD COLUMN IF NOT EXISTS incident_id text NULL;
ALTER TABLE audit_records ADD COLUMN IF NOT EXISTS alert_id text NULL;
