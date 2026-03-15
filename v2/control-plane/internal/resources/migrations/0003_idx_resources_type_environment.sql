CREATE INDEX IF NOT EXISTS idx_protected_resources_type_environment
	ON protected_resources (type, environment);
