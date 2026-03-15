CREATE INDEX IF NOT EXISTS idx_protected_resources_created_at
	ON protected_resources (created_at DESC, id DESC);
