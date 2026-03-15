CREATE INDEX IF NOT EXISTS idx_alerts_created_at
	ON alerts (created_at DESC, id DESC);
