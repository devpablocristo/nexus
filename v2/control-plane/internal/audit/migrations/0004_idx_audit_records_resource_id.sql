CREATE INDEX IF NOT EXISTS idx_audit_records_resource_id ON audit_records (resource_id) WHERE resource_id IS NOT NULL
