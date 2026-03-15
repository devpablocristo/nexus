CREATE INDEX IF NOT EXISTS idx_incidents_created_at
	ON incidents (created_at DESC, id DESC);
