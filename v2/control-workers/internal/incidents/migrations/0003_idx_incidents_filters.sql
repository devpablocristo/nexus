CREATE INDEX IF NOT EXISTS idx_incidents_filters
	ON incidents (source_kind, trigger, severity, status, archived_at, created_at DESC, id DESC);
