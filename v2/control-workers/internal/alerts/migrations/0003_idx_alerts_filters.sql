CREATE INDEX IF NOT EXISTS idx_alerts_filters
	ON alerts (source_kind, channel, severity, status, archived_at, created_at DESC, id DESC);
